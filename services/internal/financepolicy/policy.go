package financepolicy

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	AccountingCash = "CASH"
	AccountingAccrual = "ACCRUAL"
)

type Policy struct {
	DefaultPaymentTermsDays int `json:"default_payment_terms_days"`
	AccountingBasis string `json:"accounting_basis"`
	PaymentMethods []string `json:"payment_methods"`
}

func ValidateDays(days int) error {
	if days<0||days>365{return fmt.Errorf("payment terms must be between 0 and 365 days")}
	return nil
}

func ResolvePaymentTerms(platformDefault int,partnerDefault,serviceDefault,invoiceOverride *int)(int,error){
	candidates:=[]*int{invoiceOverride,serviceDefault,partnerDefault,&platformDefault}
	for _,candidate:=range candidates{
		if candidate==nil{continue}
		if err:=ValidateDays(*candidate);err!=nil{return 0,err}
		return *candidate,nil
	}
	return 0,errors.New("payment terms policy is not configured")
}

func NormalizePolicy(p Policy)(Policy,error){
	if err:=ValidateDays(p.DefaultPaymentTermsDays);err!=nil{return Policy{},err}
	p.AccountingBasis=strings.ToUpper(strings.TrimSpace(p.AccountingBasis))
	if p.AccountingBasis==""{p.AccountingBasis=AccountingCash}
	if p.AccountingBasis!=AccountingCash&&p.AccountingBasis!=AccountingAccrual{return Policy{},errors.New("accounting_basis must be CASH or ACCRUAL")}
	seen:=map[string]bool{};methods:=[]string{}
	for _,method:=range p.PaymentMethods{
		method=strings.ToUpper(strings.TrimSpace(method))
		if method==""||seen[method]{continue}
		seen[method]=true;methods=append(methods,method)
	}
	sort.Strings(methods);p.PaymentMethods=methods
	return p,nil
}
