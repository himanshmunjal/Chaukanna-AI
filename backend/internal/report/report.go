// Package report builds what the family does after a scam call: the steps to take
// and a complaint they can review and paste into cybercrime.gov.in.
//
// Chaukanna never files anything by itself. A person reads it first.
package report

import (
	"fmt"
	"strings"
	"time"

	"chaukanna/internal/models"
	"chaukanna/internal/scam"
)

// Links is where to report. These are the official Indian channels.
var Links = []Link{
	{"Call 1930 (National Cyber Crime Helpline)", "tel:1930"},
	{"File a complaint at cybercrime.gov.in", "https://cybercrime.gov.in"},
	{"Report the number on Chakshu (Sanchar Saathi)", "https://sancharsaathi.gov.in/sfc/"},
}

// Link is one reporting channel.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Report is the after-call guidance for one call.
type Report struct {
	ScamType      string   `json:"scam_type"`
	ScamTypeLabel string   `json:"scam_type_label"`
	Steps         []string `json:"steps"`
	MoneyLostStep []string `json:"money_lost_steps"`
	Links         []Link   `json:"links"`
	Draft         string   `json:"draft"`
}

// Build creates the report. The call needs its Transcript, Flags and ProtectedUser loaded.
func Build(call models.Call) Report {
	category := scam.Category(call.ScamType)
	if category == "" {
		category = scam.General
	}
	return Report{
		ScamType:      string(category),
		ScamTypeLabel: scam.CategoryLabels[category],
		Steps: []string{
			"Call them on their usual number and stay with them. Scammers often call back, or tell them not to talk to you.",
			"Do not call the number back, and do not share anything more with it.",
			"If an app such as AnyDesk was installed, uninstall it and change banking passwords from a different phone.",
			"Report the number, even if no money was lost: it protects the next person.",
		},
		MoneyLostStep: []string{
			"Call 1930 immediately. The sooner it is reported, the better the chance of freezing the money.",
			"Call the bank's official number (from the back of the card) and block the card, UPI and net banking.",
			"Keep every SMS, screenshot and transaction ID, then file at cybercrime.gov.in.",
		},
		Links: Links,
		Draft: draft(call, category),
	}
}

func draft(call models.Call, category scam.Category) string {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	var b strings.Builder
	b.WriteString("COMPLAINT DRAFT: please check every detail before submitting.\n\n")
	fmt.Fprintf(&b, "Type of fraud: %s\n", scam.CategoryLabels[category])
	fmt.Fprintf(&b, "Date and time of call: %s IST\n", call.StartedAt.In(loc).Format("2 Jan 2006, 3:04 PM"))
	if call.ProtectedUser != nil {
		fmt.Fprintf(&b, "Person who received the call: %s, %s\n", call.ProtectedUser.Name, call.ProtectedUser.Phone)
	}
	number := call.CallerNumber
	if number == "" {
		number = "[not captured: copy it from the phone's call log]"
	}
	fmt.Fprintf(&b, "Caller's number: %s\n", number)
	if call.CallerName != "" {
		fmt.Fprintf(&b, "Caller claimed to be: %s\n", call.CallerName)
	}
	b.WriteString("Money lost: [Yes/No. If yes: amount, bank, UPI/transaction ID, date and time]\n\n")

	b.WriteString("What happened:\n")
	fmt.Fprintf(&b, "I received a phone call that appears to be a %s fraud attempt.", strings.ToLower(scam.CategoryLabels[category]))
	if call.Reason != "" {
		b.WriteString(" " + call.Reason)
	}
	b.WriteString("\n")

	if len(call.Flags) > 0 {
		b.WriteString("\nWhat the caller said (speech-to-text, may contain errors):\n")
		for _, f := range call.Flags {
			fmt.Fprintf(&b, "- At %s, %s: \"%s\"\n", clock(f.TimeSec), strings.ReplaceAll(f.Type, "_", " "), f.Quote)
		}
	}
	b.WriteString("\nThis summary was prepared by the Chaukanna call-safety assistant from an automatic transcript.\n")
	return b.String()
}

func clock(sec float64) string {
	s := int(sec)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
