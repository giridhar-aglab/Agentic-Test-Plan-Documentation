package code

import (
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/model"
)

const sampleSource = `package ledger

import "errors"

// ErrInsufficientFunds is returned when a debit exceeds the balance.
var ErrInsufficientFunds = errors.New("insufficient funds")

// Account is a balance.
type Account struct {
	ID string
}

// Store persists accounts.
type Store interface {
	Load(id string) (Account, error)
}

// Post applies an entry.
func Post(account Account, amount int64, currency string) (Account, error) {
	if account.ID == "" {
		return account, errors.New("no account")
	}
	if amount == 0 || currency == "" {
		return account, errors.New("bad input")
	}
	for i := 0; i < 3; i++ {
		_ = i
	}
	return account, nil
}

// Balance is trivial.
func (a Account) Balance() int64 { return 0 }

func unexportedHelper() {}
`

func analyse(t *testing.T) FileAnalysis {
	t.Helper()
	analysis, err := AnalyseGoSource("internal/ledger/ledger.go", sampleSource)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return analysis
}

func symbolNamed(analysis FileAnalysis, name string) (model.Symbol, bool) {
	for _, symbol := range analysis.Symbols {
		if symbol.Name == name {
			return symbol, true
		}
	}
	return model.Symbol{}, false
}

func TestAnalyseGoSourceExtractsPackageAndImports(t *testing.T) {
	analysis := analyse(t)
	if analysis.PackageName != "ledger" {
		t.Errorf("expected package ledger, got %q", analysis.PackageName)
	}
	if len(analysis.Imports) != 1 || analysis.Imports[0] != "errors" {
		t.Errorf("expected the errors import, got %v", analysis.Imports)
	}
	if analysis.Depth != model.DepthTypeResolved {
		t.Errorf("go/ast gives the deepest analysis available, got %q", analysis.Depth)
	}
}

func TestAnalyseGoSourceCapturesSignatureAndErrorReturn(t *testing.T) {
	analysis := analyse(t)
	postSymbol, found := symbolNamed(analysis, "Post")
	if !found {
		t.Fatal("Post was not extracted")
	}
	if !postSymbol.Exported {
		t.Error("Post is exported")
	}
	if !postSymbol.ReturnsError {
		t.Error("a function returning error must be flagged; error paths are what scenarios test")
	}
	if postSymbol.ParameterCount != 3 {
		t.Errorf("expected 3 parameters, got %d", postSymbol.ParameterCount)
	}
	if postSymbol.Signature == "" {
		t.Error("a signature is what lets the Author write a call in the scenario steps")
	}
	if postSymbol.Ref.StartLine == 0 || postSymbol.Ref.EndLine <= postSymbol.Ref.StartLine {
		t.Errorf("a usable SourceRef needs a real line range, got %+v", postSymbol.Ref)
	}
}

func TestBranchCountReflectsTestableComplexity(t *testing.T) {
	analysis := analyse(t)
	postSymbol, _ := symbolNamed(analysis, "Post")
	balanceSymbol, _ := symbolNamed(analysis, "Balance")

	// Post has two ifs, an || and a for; Balance has nothing. The absolute
	// number matters less than the ordering, which is what drives risk scoring.
	if postSymbol.BranchCount <= balanceSymbol.BranchCount {
		t.Fatalf("branchy code must score above trivial code, got Post=%d Balance=%d",
			postSymbol.BranchCount, balanceSymbol.BranchCount)
	}
	if analysis.ComplexityScore == 0 {
		t.Fatal("the file complexity score must aggregate its symbols")
	}
}

func TestAnalyseGoSourceDistinguishesKinds(t *testing.T) {
	analysis := analyse(t)

	if accountSymbol, found := symbolNamed(analysis, "Account"); !found || accountSymbol.Kind != "type" {
		t.Errorf("Account should be a type, got %+v", accountSymbol)
	}
	if storeSymbol, found := symbolNamed(analysis, "Store"); !found || storeSymbol.Kind != "interface" {
		t.Errorf("Store should be an interface; interfaces are the seams tests mock at, got %+v", storeSymbol)
	}
	balanceSymbol, found := symbolNamed(analysis, "Balance")
	if !found || balanceSymbol.Kind != "method" || balanceSymbol.Receiver != "Account" {
		t.Errorf("Balance should be a method on Account, got %+v", balanceSymbol)
	}
	if balanceSymbol.Ref.Symbol != "Account.Balance" {
		t.Errorf("a method's ref should be qualified so traceability is unambiguous, got %q",
			balanceSymbol.Ref.Symbol)
	}
	if helperSymbol, found := symbolNamed(analysis, "unexportedHelper"); !found || helperSymbol.Exported {
		t.Errorf("unexported symbols are still extracted but must not be marked exported, got %+v", helperSymbol)
	}
}

func TestAnalyseGoSourceReportsSyntaxErrors(t *testing.T) {
	// A broken file is the repository's problem, not the model's: the caller
	// records a gap and moves on rather than failing the run.
	if _, err := AnalyseGoSource("broken.go", "package ledger\nfunc ("); err == nil {
		t.Fatal("expected a parse error for malformed source")
	}
}
