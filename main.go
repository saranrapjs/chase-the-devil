package main

import (
	"encoding/csv"
	"errors"
	"flag"
	l "log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Transaction represents a basic statement transaction, as shown by Chase in their Credit Card PDF statements.
type Transaction struct {
	Amount       float64
	MerchantName string
	Date         time.Time
}

var log = l.New(os.Stderr, "", l.LstdFlags)

// Values exports an individual Transaction in a CSV-friendly way — this format is derived from the CSV
// format (including the Y/M/D style) you get when exporting transactions from Chase.
func (t *Transaction) Values() []string {
	var tType string
	switch {
	case t.Amount < 0:
		tType = "Payment"
	default:
		tType = "Sale"
	}
	return []string{
		tType, // "Type"
		t.Date.Format("01/02/2006"),                    // "Trans Date"
		t.Date.Format("01/02/2006"),                    // "Post Date"
		t.MerchantName,                                 // "Description"
		strconv.FormatFloat(-1.0*t.Amount, 'f', 2, 64), // "Amount"
	}
}

// Transactions represents a series of transactions, aliased this way for chronological date sorting.
type Transactions []Transaction

func (t Transactions) Len() int {
	return len(t)
}

func (t Transactions) Less(i, j int) bool {
	return t[j].Date.Before(t[i].Date)
}

func (t Transactions) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Statement reprents a list of transactions, plus some sum-oriented metadata used
// for confirming the validity of the parsed transaction amounts.
type Statement struct {
	Transactions    Transactions
	StartingBalance float64
	EndingBalance   float64
}

// Headers returns CSV friendly versions of the Transaction-level field names.
func (s *Statement) Headers() []string {
	return []string{
		"Type",
		"Trans Date",
		"Post Date",
		"Description",
		"Amount",
	}
}

// Reconcile will check the sum of all the statement amounts against the parsed
// starting and ending balances.
func (s *Statement) Reconcile() (float64, bool) {
	var isOK bool
	total := s.StartingBalance
	for _, t := range s.Transactions {
		total += t.Amount
	}
	if toFixed(total, 2) == s.EndingBalance {
		isOK = true
	}
	return total, isOK
}

var findStatements = regexp.MustCompile(`(?m)^([0-9]{0,2})/([0-9]{0,2}) (.*) ([0-9\-\.,]+)`)

var findEnd = regexp.MustCompile(`Amount Rewards`)

var (
	findPreviousBalance = regexp.MustCompile(`(?m)^Previous Balance \$([0-9\-\.,]+)`)
	findNewBalance      = regexp.MustCompile(`(?m)^New Balance \$([0-9\-\.,]+)`)
	findYear            = regexp.MustCompile(`(?m)^([0-9]{0,4}) Totals Year-to-Date`)
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("missing file path")
	}

	file := args[0]

	body, _ := exec.Command("pdftotext", "-raw", "-nopgbrk", file, "-").Output()
	var statement Statement

	var yearBytes []byte
	if yb := findYear.FindSubmatch(body); yb != nil {
		yearBytes = yb[1]
	}

        end := len(body)
	if loc := findEnd.FindIndex(body); loc != nil {
		end = loc[0]
        }
	sts := findStatements.FindAllSubmatch(body[0:end], -1)
	log.Printf("Found %d matches\n", len(sts))
	for i, st := range sts {
		if len(st) < 4 {
			log.Fatalf("Bad match for match no %d, aborting\n", i)
			continue
		}
		var t Transaction
		t.MerchantName = string(st[3])
		amt, err := sanitizeAmount(string(st[4]))
		if err != nil {
			log.Fatalf("Bad amount parse for \"%s\": %v, aborting\n", t.MerchantName, err)
			continue
		}
		t.Amount = amt
		d, err := createDate(st[2], st[1], yearBytes)
		if err != nil {
			log.Fatalf("Bad date parse for \"%s\": %v, aborting\n", t.MerchantName, err)
		}
		t.Date = d
		statement.Transactions = append(statement.Transactions, t)
	}
	if start := findPreviousBalance.FindSubmatch(body); start != nil {
		amt, err := sanitizeAmount(string(start[1]))
		if err != nil {
			log.Fatal("error with Previous Balance:", err)
		}
		statement.StartingBalance = amt
	} else {
		log.Fatal("could not find starting balance :( ")
	}
	if end := findNewBalance.FindSubmatch(body); end != nil {
		amt, err := sanitizeAmount(string(end[1]))
		if err != nil {
			log.Fatal("error with New Balance:", err)
		}
		statement.EndingBalance = amt
	} else {
		log.Fatal("could not find ending balance :( ")
	}
	sort.Sort(statement.Transactions)
	if val, res := statement.Reconcile(); !res {
		log.Fatalf("reconciliation doesn't match :(, actual: %v, expected %v", val, statement.EndingBalance)
	}
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()
	writer.Write(statement.Headers())
	for _, tr := range statement.Transactions {
		writer.Write(tr.Values())
	}
}

func sanitizeAmount(amtString string) (float64, error) {
	amtString = strings.Replace(amtString, ",", "", -1)
	num, err := strconv.ParseFloat(amtString, 64)
	if err != nil {
		return 0.0, err
	}
	return toFixed(num, 2), nil
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func createDate(day, month, year []byte) (time.Time, error) {
	var t time.Time
	var d, m, y int
	if day == nil || month == nil || year == nil {
		return t, errors.New("a piece of the date is missing")
	}

	d, err := strconv.Atoi(string(day))
	if err != nil {
		return t, err
	}

	m, err = strconv.Atoi(string(month))
	if err != nil {
		return t, err
	}

	y, err = strconv.Atoi(string(year))
	if err != nil {
		return t, err
	}

	t = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
	return t, nil
}
