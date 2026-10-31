package images

// Test-only exports for the external journal tests.
var (
	TestFixture  = fixture
	TestAnalysis = analysis
	TestEncode   = encode
)

type TestProvider = provider

func (p *provider) Calls() int { return p.calls }
