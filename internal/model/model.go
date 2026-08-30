// Package model holds the domain types that agents exchange.
//
// Agents never hand each other conversation transcripts. They hand each other
// these values, serialised as JSON and stored on the blackboard. That is what
// keeps context bounded and every handoff testable with an ordinary unit test.
package model

import "time"

// Language identifies an analysable source language.
type Language string

const (
	LanguageGo      Language = "go"
	LanguagePython  Language = "python"
	LanguageUnknown Language = "unknown"
)

// AnalysisDepth records how thoroughly a component could be understood. It
// travels with every finding so the final report can be honest about which
// conclusions rest on full type resolution and which rest on a guess.
type AnalysisDepth string

const (
	// DepthTypeResolved means a real compiler front end resolved the symbols.
	DepthTypeResolved AnalysisDepth = "type-resolved"
	// DepthSyntactic means the structure is known but types are not.
	DepthSyntactic AnalysisDepth = "syntactic"
	// DepthInferred means a model read the source and described it.
	DepthInferred AnalysisDepth = "inferred"
)

// SourceRef anchors a finding to a location in the analysed commit. Every
// scenario carries one, and report validation fails if it does not resolve to
// a symbol that actually exists.
type SourceRef struct {
	Path      string `json:"path"`
	Symbol    string `json:"symbol,omitempty"`
	StartLine int    `json:"startLine,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
}

// SourceFile is one file in the repository under analysis.
type SourceFile struct {
	Path            string   `json:"path"`
	BlobSHA         string   `json:"blobSha"`
	SizeBytes       int64    `json:"sizeBytes"`
	Language        Language `json:"language"`
	IsTest          bool     `json:"isTest"`
	IsGenerated     bool     `json:"isGenerated"`
	IsVendored      bool     `json:"isVendored"`
	ExcludedReason  string   `json:"excludedReason,omitempty"`
	SelectedForScan bool     `json:"selectedForScan"`
}

// Analysable reports whether the file should be sent to the Analyst.
func (sourceFile SourceFile) Analysable() bool {
	if sourceFile.IsTest || sourceFile.IsGenerated || sourceFile.IsVendored {
		return false
	}
	return sourceFile.Language != LanguageUnknown
}

// Dependency is an external package the code relies on.
type Dependency struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Language     Language `json:"language"`
	Direct       bool     `json:"direct"`
	RequiresMock bool     `json:"requiresMock"`
	Reason       string   `json:"reason,omitempty"`
}

// ExistingTest is a test already present in the repository.
type ExistingTest struct {
	Path         string    `json:"path"`
	TestName     string    `json:"testName"`
	CoversRef    SourceRef `json:"coversRef"`
	MatchedByAST bool      `json:"matchedByAst"`
}

// RepoMap is the Surveyor's output: what this repository is made of, before
// anyone has reasoned about behaviour.
type RepoMap struct {
	RepositoryName    string         `json:"repositoryName"`
	CommitSHA         string         `json:"commitSha"`
	Languages         []Language     `json:"languages"`
	Files             []SourceFile   `json:"files"`
	Dependencies      []Dependency   `json:"dependencies"`
	ExistingTests     []ExistingTest `json:"existingTests"`
	Entrypoints       []SourceRef    `json:"entrypoints"`
	BuildSystemNotes  string         `json:"buildSystemNotes,omitempty"`
	TestFrameworkName string         `json:"testFrameworkName,omitempty"`
	SurveyedAt        time.Time      `json:"surveyedAt"`
}

// SelectedFiles returns the files chosen for analysis, in listing order.
func (repoMap RepoMap) SelectedFiles() []SourceFile {
	selectedFiles := make([]SourceFile, 0, len(repoMap.Files))
	for _, candidateFile := range repoMap.Files {
		if candidateFile.SelectedForScan {
			selectedFiles = append(selectedFiles, candidateFile)
		}
	}
	return selectedFiles
}

// Symbol is one exported declaration the Analyst may reason about.
type Symbol struct {
	Name           string    `json:"name"`
	Kind           string    `json:"kind"` // func, method, type, const, var
	Receiver       string    `json:"receiver,omitempty"`
	Signature      string    `json:"signature,omitempty"`
	DocComment     string    `json:"docComment,omitempty"`
	Ref            SourceRef `json:"ref"`
	Exported       bool      `json:"exported"`
	ReturnsError   bool      `json:"returnsError"`
	BranchCount    int       `json:"branchCount"`
	ParameterCount int       `json:"parameterCount"`
}

// ComponentModel is the Analyst's output for one unit of code.
type ComponentModel struct {
	ComponentName   string        `json:"componentName"`
	Path            string        `json:"path"`
	BlobSHA         string        `json:"blobSha"`
	Language        Language      `json:"language"`
	Depth           AnalysisDepth `json:"depth"`
	Responsibility  string        `json:"responsibility"`
	PublicSymbols   []Symbol      `json:"publicSymbols"`
	ExternalCalls   []Dependency  `json:"externalCalls"`
	ErrorPaths      []string      `json:"errorPaths"`
	SideEffects     []string      `json:"sideEffects"`
	ComplexityScore int           `json:"complexityScore"`
	Confidence      float64       `json:"confidence"`
	AnalysedAt      time.Time     `json:"analysedAt"`
}

// RiskLevel orders the risk register and drives scenario priority.
type RiskLevel string

const (
	RiskCritical RiskLevel = "critical"
	RiskHigh     RiskLevel = "high"
	RiskMedium   RiskLevel = "medium"
	RiskLow      RiskLevel = "low"
)

// Risk is one entry in the prioritised register.
type Risk struct {
	ID              string    `json:"id"`
	ComponentName   string    `json:"componentName"`
	Ref             SourceRef `json:"ref"`
	Level           RiskLevel `json:"level"`
	Score           float64   `json:"score"`
	Rationale       string    `json:"rationale"`
	Signals         []string  `json:"signals"`
	HasExistingTest bool      `json:"hasExistingTest"`
}

// RiskRegister is the Risk agent's output.
type RiskRegister struct {
	Risks       []Risk    `json:"risks"`
	ScoredAt    time.Time `json:"scoredAt"`
	ScoringNote string    `json:"scoringNote,omitempty"`
}

// Prioritised returns the risks that must each receive at least one scenario.
func (riskRegister RiskRegister) Prioritised() []Risk {
	prioritisedRisks := make([]Risk, 0, len(riskRegister.Risks))
	for _, candidateRisk := range riskRegister.Risks {
		if candidateRisk.Level == RiskCritical || candidateRisk.Level == RiskHigh {
			prioritisedRisks = append(prioritisedRisks, candidateRisk)
		}
	}
	return prioritisedRisks
}

// ScenarioType classifies what kind of test a scenario describes.
type ScenarioType string

const (
	ScenarioUnit        ScenarioType = "unit"
	ScenarioIntegration ScenarioType = "integration"
	ScenarioContract    ScenarioType = "contract"
	ScenarioEndToEnd    ScenarioType = "e2e"
	ScenarioNegative    ScenarioType = "negative"
	ScenarioBoundary    ScenarioType = "boundary"
	ScenarioPerformance ScenarioType = "performance"
	ScenarioSecurity    ScenarioType = "security"
)

// Priority mirrors the linked risk level.
type Priority string

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
)

// PriorityForRisk maps a risk level onto a scenario priority.
func PriorityForRisk(riskLevel RiskLevel) Priority {
	switch riskLevel {
	case RiskCritical:
		return PriorityP0
	case RiskHigh:
		return PriorityP1
	case RiskMedium:
		return PriorityP2
	default:
		return PriorityP3
	}
}

// Step is one action in a scenario.
type Step struct {
	Ordinal int    `json:"ordinal"`
	Action  string `json:"action"`
}

// DataRequirement describes test data a scenario needs.
type DataRequirement struct {
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
}

// TestScenario is the unit the whole system exists to produce.
type TestScenario struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	ComponentName  string            `json:"componentName"`
	SourceRef      SourceRef         `json:"sourceRef"`
	Type           ScenarioType      `json:"type"`
	Priority       Priority          `json:"priority"`
	RiskRef        string            `json:"riskRef"`
	Preconditions  []string          `json:"preconditions"`
	Steps          []Step            `json:"steps"`
	ExpectedResult string            `json:"expectedResult"`
	TestData       []DataRequirement `json:"testData,omitempty"`
	Mocks          []Dependency      `json:"mocks,omitempty"`
	Automatable    bool              `json:"automatable"`
	Confidence     float64           `json:"confidence"`
	Notes          string            `json:"notes,omitempty"`
}

// Decision is a reviewer's judgement at the approval gate.
type Decision string

const (
	DecisionAccept Decision = "accept"
	DecisionEdit   Decision = "edit"
	DecisionReject Decision = "reject"
)

// Verdict is recorded at the approval gate. It is the unit of both publication
// and learning: an edit carries the original alongside the kept version, and
// that diff is the most useful training signal the system ever sees.
type Verdict struct {
	ScenarioID string        `json:"scenarioId"`
	Decision   Decision      `json:"decision"`
	EditedFrom *TestScenario `json:"editedFrom,omitempty"`
	Comment    string        `json:"comment,omitempty"`
	ReviewedBy string        `json:"reviewedBy,omitempty"`
	ReviewedAt time.Time     `json:"reviewedAt"`
	JiraKey    string        `json:"jiraKey,omitempty"`
}

// ProvenanceKind records where a finding came from, so degraded runs stay
// legible in the final report.
type ProvenanceKind string

const (
	ProvenanceInternal ProvenanceKind = "internal"
	ProvenanceMCP      ProvenanceKind = "mcp"
	ProvenanceExternal ProvenanceKind = "external"
)

// Provenance names the source of a result.
type Provenance struct {
	Kind   ProvenanceKind `json:"kind"`
	Server string         `json:"server,omitempty"`
}

func (provenance Provenance) String() string {
	if provenance.Server == "" {
		return string(provenance.Kind)
	}
	return string(provenance.Kind) + ":" + provenance.Server
}

// ReviewSeverity orders the Critic's findings.
type ReviewSeverity string

const (
	ReviewBlocking ReviewSeverity = "blocking"
	ReviewMajor    ReviewSeverity = "major"
	ReviewMinor    ReviewSeverity = "minor"
)

// RevisionRequest is one Critic finding. The Author gets these back, and the
// bounded revision cycle requires the count to strictly decrease each round —
// so a critic that keeps finding the same thing ends the loop rather than
// extending it.
type RevisionRequest struct {
	ScenarioID string         `json:"scenarioId,omitempty"`
	RiskRef    string         `json:"riskRef,omitempty"`
	Severity   ReviewSeverity `json:"severity"`
	Issue      string         `json:"issue"`
	Suggestion string         `json:"suggestion,omitempty"`
}

// Blocking reports whether this finding must be addressed before review.
func (revisionRequest RevisionRequest) Blocking() bool {
	return revisionRequest.Severity == ReviewBlocking
}
