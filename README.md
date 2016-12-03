### Chase the devil

> I'm gonna put on a iron shirt, and chase satan out of earth

This tool derives a CSV of transactions from Chase's credit card statement PDF's. I created it because Chase limits its electronically-downloadable history to the last 5 months or so, while the statement PDF's exist for a longer period of time.

### Requirements

[`go`](https://golang.org) and the `pdftotext` command line utility.

### Install

```bash
go install .
```

### How to run

```bash
chase-the-devil [PATH-TO-PDF-FILE] > output.csv
```
