package financepolicy

import "testing"

func intPtr(v int)*int{return &v}

func TestPaymentTermsArePolicyNotHardCoded(t *testing.T){
	got,err:=ResolvePaymentTerms(8,intPtr(15),nil,nil);if err!=nil||got!=15{t.Fatalf("partner default must override platform fallback: %d %v",got,err)}
	got,err=ResolvePaymentTerms(8,intPtr(15),intPtr(30),nil);if err!=nil||got!=30{t.Fatalf("service default must override partner default: %d %v",got,err)}
	got,err=ResolvePaymentTerms(8,intPtr(15),intPtr(30),intPtr(3));if err!=nil||got!=3{t.Fatalf("invoice override must win: %d %v",got,err)}
}

func TestFinancePolicySupportsCashAndAccrual(t *testing.T){
	for _,basis:=range []string{"CASH","ACCRUAL"}{
		p,err:=NormalizePolicy(Policy{DefaultPaymentTermsDays:8,AccountingBasis:basis,PaymentMethods:[]string{"stripe","ZELLE","stripe","cash"}})
		if err!=nil{t.Fatal(err)}
		if p.AccountingBasis!=basis||len(p.PaymentMethods)!=3{t.Fatalf("unexpected normalized policy: %#v",p)}
	}
}
