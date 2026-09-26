package main

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

const central14CompanyDocumentScope = "_himate"

func (a *app) companyDocuments(w http.ResponseWriter, r *http.Request) {
	a.documents(w,r,central14CompanyDocumentScope)
}

func nullableCentral14Time(value sql.NullTime) any {
	if value.Valid { return value.Time.UTC() }
	return nil
}

func (a *app) central14AdministrationSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use GET")
		return
	}
	type aggregate struct {
		Documents int
		Invoices int
		LastDocument sql.NullTime
		LastInvoice sql.NullTime
	}
	byPartner:=map[string]*aggregate{}
	get:=func(id string)*aggregate{
		row:=byPartner[id]
		if row==nil { row=&aggregate{};byPartner[id]=row }
		return row
	}

	docRows,err:=a.db.Query(`SELECT partner_id,COUNT(*),MAX(created_at)
		FROM billing.documents GROUP BY partner_id`)
	if err!=nil { common.APIError(w,500,"DB","Could not load administration document summary");return }
	for docRows.Next(){
		var id string;var count int;var latest sql.NullTime
		if docRows.Scan(&id,&count,&latest)==nil{
			row:=get(id);row.Documents=count;row.LastDocument=latest
		}
	}
	docRows.Close()

	invoiceRows,err:=a.db.Query(`SELECT partner_id,COUNT(*),MAX(created_at)
		FROM billing.invoices GROUP BY partner_id`)
	if err!=nil { common.APIError(w,500,"DB","Could not load administration invoice summary");return }
	for invoiceRows.Next(){
		var id string;var count int;var latest sql.NullTime
		if invoiceRows.Scan(&id,&count,&latest)==nil{
			row:=get(id);row.Invoices=count;row.LastInvoice=latest
		}
	}
	invoiceRows.Close()

	items:=make([]map[string]any,0,len(byPartner))
	for id,row:=range byPartner{
		if id==central14CompanyDocumentScope { continue }
		items=append(items,map[string]any{
			"partner_id":id,
			"document_count":row.Documents,
			"invoice_count":row.Invoices,
			"last_document_at":nullableCentral14Time(row.LastDocument),
			"last_invoice_at":nullableCentral14Time(row.LastInvoice),
		})
	}
	company:=get(central14CompanyDocumentScope)
	common.JSON(w,http.StatusOK,map[string]any{
		"items":items,
		"company":map[string]any{
			"scope":central14CompanyDocumentScope,
			"document_count":company.Documents,
			"last_document_at":nullableCentral14Time(company.LastDocument),
		},
		"generated_at":time.Now().UTC(),
		"source":"BILLING_DOCUMENTS_INVOICES",
	})
}

func central14DocumentSearch(raw string) string {
	return "%" + strings.ToLower(strings.TrimSpace(raw)) + "%"
}
