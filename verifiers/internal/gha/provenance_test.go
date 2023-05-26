package gha

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"

	serrors "github.com/slsa-framework/slsa-verifier/v2/errors"
	"github.com/slsa-framework/slsa-verifier/v2/verifiers/internal/gha/slsaprovenance"
	"github.com/slsa-framework/slsa-verifier/v2/verifiers/internal/gha/slsaprovenance/iface"
)

type testProvenance struct {
	builderID         string
	sourceURI         string
	triggerURI        string
	subjects          []intoto.Subject
	branch            string
	tag               string
	buildTriggerPath  string
	systemParameters  map[string]any
	buildInvocationID string
	buildStartTime    *time.Time
	buildFinishTime   *time.Time
	noResolvedDeps    int
	workflowInputs    map[string]any
}

func (p *testProvenance) BuilderID() (string, error)           { return p.builderID, nil }
func (p *testProvenance) SourceURI() (string, error)           { return p.sourceURI, nil }
func (p *testProvenance) TriggerURI() (string, error)          { return p.triggerURI, nil }
func (p *testProvenance) Subjects() ([]intoto.Subject, error)  { return p.subjects, nil }
func (p *testProvenance) GetBranch() (string, error)           { return p.branch, nil }
func (p *testProvenance) GetTag() (string, error)              { return p.tag, nil }
func (p *testProvenance) GetBuildTriggerPath() (string, error) { return p.buildTriggerPath, nil }
func (p *testProvenance) GetSystemParameters() (map[string]any, error) {
	return p.systemParameters, nil
}
func (p *testProvenance) GetBuildInvocationID() (string, error)       { return p.buildInvocationID, nil }
func (p *testProvenance) GetBuildStartTime() (*time.Time, error)      { return p.buildStartTime, nil }
func (p *testProvenance) GetBuildFinishTime() (*time.Time, error)     { return p.buildFinishTime, nil }
func (p *testProvenance) GetNumberResolvedDependencies() (int, error) { return p.noResolvedDeps, nil }
func (p *testProvenance) GetWorkflowInputs() (map[string]interface{}, error) {
	return p.workflowInputs, nil
}

type testProvenanceV02 struct {
	testProvenance
	predicate slsa02.ProvenancePredicate
}

func (p *testProvenanceV02) Predicate() slsa02.ProvenancePredicate { return p.predicate }

type testProvenanceV1 struct {
	testProvenance
	predicate slsa1.ProvenancePredicate
}

func (p *testProvenanceV1) Predicate() slsa1.ProvenancePredicate { return p.predicate }

func provenanceFromBytes(payload []byte) (iface.Provenance, error) {
	env, err := EnvelopeFromBytes(payload)
	if err != nil {
		return nil, err
	}
	return slsaprovenance.ProvenanceFromEnvelope(env)
}

func Test_ProvenanceFromEnvelope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		expected error
	}{
		{
			name:     "invalid dsse: not SLSA predicate",
			path:     "./testdata/dsse-not-slsa.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		{
			name:     "slsa 1.0 invalid dsse: not SLSA predicate",
			path:     "./testdata/dsse-not-slsa-v1.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		// TODO(#573): add more copliance tests.
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.path)
			if err != nil {
				panic(fmt.Errorf("os.ReadFile: %w", err))
			}
			_, err = provenanceFromBytes(content)
			if !errCmp(err, tt.expected) {
				t.Errorf(cmp.Diff(err, tt.expected))
			}
		})
	}
}

func Test_VerifyDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		path         string
		artifactHash string
		expected     error
	}{
		{
			name:         "invalid dsse: no sha256 subject digest",
			path:         "./testdata/dsse-no-subject-hash.intoto.jsonl",
			artifactHash: "0ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorInvalidDssePayload,
		},
		{
			name:     "invalid dsse: nil subject",
			path:     "./testdata/dsse-no-subject.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		{
			name:         "mismatched artifact hash with env",
			path:         "./testdata/dsse-valid.intoto.jsonl",
			artifactHash: "1ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorMismatchHash,
		},
		{
			name:         "valid entry",
			path:         "./testdata/dsse-valid.intoto.jsonl",
			artifactHash: "0ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},
		{
			name:         "valid entry multiple subjects last entry",
			path:         "./testdata/dsse-valid-multi-subjects.intoto.jsonl",
			artifactHash: "03e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},
		{
			name:         "valid multiple subjects second entry",
			path:         "./testdata/dsse-valid-multi-subjects.intoto.jsonl",
			artifactHash: "02e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},
		{
			name:         "multiple subjects invalid hash",
			path:         "./testdata/dsse-valid-multi-subjects.intoto.jsonl",
			artifactHash: "04e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorMismatchHash,
		},
		{
			name:         "invalid dsse: nil subject",
			path:         "./testdata/dsse-no-subject-v1.intoto.jsonl",
			artifactHash: "0ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorInvalidDssePayload,
		},
		{
			name:         "invalid dsse: no sha256 subject digest",
			path:         "./testdata/dsse-no-subject-hash-v1.intoto.jsonl",
			artifactHash: "0ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorInvalidDssePayload,
		},
		{
			name:         "mismatched artifact hash with env",
			path:         "./testdata/dsse-valid-v1.intoto.jsonl",
			artifactHash: "1ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorMismatchHash,
		},

		{
			name:         "valid entry",
			path:         "./testdata/dsse-valid-v1.intoto.jsonl",
			artifactHash: "0ae7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},

		{
			name:         "valid entry multiple subjects last entry",
			path:         "./testdata/dsse-valid-multi-subjects-v1.intoto.jsonl",
			artifactHash: "03e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},
		{
			name:         "valid multiple subjects second entry",
			path:         "./testdata/dsse-valid-multi-subjects-v1.intoto.jsonl",
			artifactHash: "02e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     nil,
		},
		{
			name:         "multiple subjects invalid hash",
			path:         "./testdata/dsse-valid-multi-subjects-v1.intoto.jsonl",
			artifactHash: "04e7e4fa71686538440012ee36a2634dbaa19df2dd16a466f52411fb348bbc4e",
			expected:     serrors.ErrorMismatchHash,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.path)
			if err != nil {
				panic(fmt.Errorf("os.ReadFile: %w", err))
			}
			prov, err := provenanceFromBytes(content)
			if err != nil {
				panic(fmt.Errorf("provenanceFromBytes: %w", err))
			}

			err = verifyDigest(prov, tt.artifactHash)
			if !errCmp(err, tt.expected) {
				t.Errorf(cmp.Diff(err, tt.expected))
			}
		})
	}
}

func Test_verifySourceURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		provMaterialsURI   string
		provTriggerURI     string
		expectedSourceURI  string
		allowNoMaterialRef bool
		err                error
	}{
		{
			name:              "source has no @",
			provMaterialsURI:  "git+https://github.com/some/repo",
			provTriggerURI:    "git+https://github.com/some/repo",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "empty materials",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorInvalidDssePayload,
		},
		{
			name:              "empty configSource",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "empty uri materials",
			provMaterialsURI:  " ",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "no tag uri materials",
			provTriggerURI:    "git+https://github.com/some/repo",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "no tag uri configSource",
			provMaterialsURI:  "git+https://github.com/some/repo",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "match source",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
		},
		{
			name:              "match source no git",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "https://github.com/some/repo",
		},
		{
			name:               "match source no git no material ref",
			provTriggerURI:     "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:   "git+https://github.com/some/repo",
			allowNoMaterialRef: true,
			expectedSourceURI:  "https://github.com/some/repo",
		},
		{
			name:              "match source no git no material ref ref not allowed",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo",
			expectedSourceURI: "https://github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "match source no git+https",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "github.com/some/repo",
		},
		{
			name:              "match source no repo",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "some/repo",
			err:               serrors.ErrorMalformedURI,
		},
		{
			name:              "mismatch materials configSource tag",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.4",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorInvalidDssePayload,
		},
		{
			name:              "mismatch materials configSource org",
			provTriggerURI:    "git+https://github.com/other/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMismatchSource,
		},
		{
			name:              "mismatch configSource materials org",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/other/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMismatchSource,
		},
		{
			name:              "mismatch materials configSource name",
			provTriggerURI:    "git+https://github.com/some/other@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMismatchSource,
		},
		{
			name:              "mismatch configSource materials name",
			provTriggerURI:    "git+https://github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://github.com/some/other@v1.2.3",
			expectedSourceURI: "git+https://github.com/some/repo",
			err:               serrors.ErrorMismatchSource,
		},
		{
			name:              "not github.com repo",
			provTriggerURI:    "git+https://not-github.com/some/repo@v1.2.3",
			provMaterialsURI:  "git+https://not-github.com/some/repo@v1.2.3",
			expectedSourceURI: "git+https://not-github.com/some/repo",
			err:               serrors.ErrorMalformedURI,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prov02 := &testProvenance{
				sourceURI:  tt.provMaterialsURI,
				triggerURI: tt.provTriggerURI,
			}

			err := verifySourceURI(prov02, tt.expectedSourceURI, tt.allowNoMaterialRef)
			if !errCmp(err, tt.err) {
				t.Errorf(cmp.Diff(err, tt.err))
			}
		})
	}
}

func Test_verifyBuilderIDExactMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		builderID  string
		expectedID string
		err        error
	}{
		{
			name:       "match no version",
			builderID:  "some/builderID",
			expectedID: "some/builderID",
		},
		{
			name:       "match with tag",
			builderID:  "some/builderID@v1.2.3",
			expectedID: "some/builderID@v1.2.3",
		},
		{
			name:       "same builderID mismatch version",
			builderID:  "some/builderID@v1.2.3",
			expectedID: "some/builderID@v1.2.4",
			err:        serrors.ErrorMismatchBuilderID,
			// TODO(#189): this should fail.
		},
		{
			name:       "mismatch builderID same version",
			builderID:  "tome/builderID@v1.2.3",
			expectedID: "some/builderID@v1.2.3",
			err:        serrors.ErrorMismatchBuilderID,
		},
		{
			name:       "empty prov builderID",
			builderID:  "",
			expectedID: "some/builderID",
			err:        serrors.ErrorMismatchBuilderID,
		},
		{
			name:       "empty expected builderID",
			builderID:  "tome/builderID@v1.2.3",
			expectedID: "",
			err:        serrors.ErrorMismatchBuilderID,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prov := &testProvenance{
				builderID: tt.builderID,
			}

			err := verifyBuilderIDExactMatch(prov, tt.expectedID)
			if !errCmp(err, tt.err) {
				t.Errorf(cmp.Diff(err, tt.err))
			}
		})
	}
}

func Test_VerifyBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		branch   string
		expected error
	}{
		{
			name:   "ref main",
			path:   "./testdata/dsse-main-ref.intoto.jsonl",
			branch: "main",
		},
		{
			name:   "ref branch3",
			path:   "./testdata/dsse-branch3-ref.intoto.jsonl",
			branch: "branch3",
		},
		{
			name:     "invalid ref type",
			path:     "./testdata/dsse-invalid-ref-type.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		{
			name:   "tag branch2 push trigger",
			path:   "./testdata/dsse-branch2-tag.intoto.jsonl",
			branch: "branch2",
		},
		{
			name:   "v10.0.1 release trigger",
			path:   "./testdata/dsse-v10.0.1-release.intoto.jsonl",
			branch: "main",
		},
		{
			name:     "from commit push - no branch",
			path:     "./testdata/dsse-annotated-tag.intoto.jsonl",
			branch:   "main",
			expected: serrors.ErrorMismatchBranch,
		},

		{
			name:   "ref main",
			path:   "./testdata/dsse-main-ref-v1.intoto.jsonl",
			branch: "main",
		},
		{
			name:     "ref main case-sensitive",
			path:     "./testdata/dsse-main-ref-v1.intoto.jsonl",
			branch:   "Main",
			expected: serrors.ErrorMismatchBranch,
		},

		{
			name:     "invalid ref type",
			path:     "./testdata/dsse-invalid-ref-type-v1.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},

		{
			name:   "tag branch2 push trigger",
			path:   "./testdata/dsse-branch2-tag-v1.intoto.jsonl",
			branch: "branch2",
		},

		{
			name:   "v10.0.1 release trigger",
			path:   "./testdata/dsse-v10.0.1-release-v1.intoto.jsonl",
			branch: "main",
		},

		{
			name:     "from commit push - no branch",
			path:     "./testdata/dsse-annotated-tag-v1.intoto.jsonl",
			branch:   "main",
			expected: serrors.ErrorMismatchBranch,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.path)
			if err != nil {
				panic(fmt.Errorf("os.ReadFile: %w", err))
			}
			prov, err := provenanceFromBytes(content)
			if err != nil {
				panic(fmt.Errorf("provenanceFromBytes: %w", err))
			}

			err = VerifyBranch(prov, tt.branch)
			if !errCmp(err, tt.expected) {
				t.Errorf(cmp.Diff(err, tt.expected))
			}
		})
	}
}

func Test_VerifyWorkflowInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		inputs   map[string]string
		expected error
	}{
		{
			name: "match all",
			path: "./testdata/dsse-workflow-inputs.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_bool":       "true",
				"some_integer":    "123",
			},
		},
		{
			name: "match subset",
			path: "./testdata/dsse-workflow-inputs.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_integer":    "123",
			},
		},
		{
			name: "missing field",
			path: "./testdata/dsse-workflow-inputs.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"missing_field":   "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "mismatch field release_version",
			path: "./testdata/dsse-workflow-inputs.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.4",
				"some_integer":    "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "mismatch field some_integer",
			path: "./testdata/dsse-workflow-inputs.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_integer":    "124",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "not workflow_dispatch trigger",
			path: "./testdata/dsse-workflow-inputs-wrong-trigger.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_bool":       "true",
				"some_integer":    "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "match all",
			path: "./testdata/dsse-workflow-inputs-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_bool":       "true",
				"some_integer":    "123",
			},
		},
		{
			name: "match subset",
			path: "./testdata/dsse-workflow-inputs-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_integer":    "123",
			},
		},
		{
			name: "missing field",
			path: "./testdata/dsse-workflow-inputs-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"missing_field":   "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "mismatch field release_version",
			path: "./testdata/dsse-workflow-inputs-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.4",
				"some_integer":    "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "mismatch field some_integer",
			path: "./testdata/dsse-workflow-inputs-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_integer":    "124",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
		{
			name: "not workflow_dispatch trigger",
			path: "./testdata/dsse-workflow-inputs-wrong-trigger-v1.intoto.jsonl",
			inputs: map[string]string{
				"release_version": "v1.2.3",
				"some_bool":       "true",
				"some_integer":    "123",
			},
			expected: serrors.ErrorMismatchWorkflowInputs,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.path)
			if err != nil {
				panic(fmt.Errorf("os.ReadFile: %w", err))
			}
			prov, err := provenanceFromBytes(content)
			if err != nil {
				panic(fmt.Errorf("provenanceFromBytes: %w", err))
			}

			err = VerifyWorkflowInputs(prov, tt.inputs)
			if !errCmp(err, tt.expected) {
				t.Errorf(cmp.Diff(err, tt.expected))
			}
		})
	}
}

func Test_VerifyTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		tag      string
		expected error
	}{
		{
			name:     "ref main",
			path:     "./testdata/dsse-main-ref.intoto.jsonl",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name:     "ref branch3",
			path:     "./testdata/dsse-branch3-ref.intoto.jsonl",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name:     "invalid ref type",
			path:     "./testdata/dsse-invalid-ref-type.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		{
			name: "tag vslsa1",
			path: "./testdata/dsse-vslsa1-tag.intoto.jsonl",
			tag:  "vslsa1",
		},
		{
			name:     "ref branch3",
			path:     "./testdata/dsse-branch3-ref-v1.intoto.jsonl",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name:     "ref main",
			path:     "./testdata/dsse-main-ref-v1.intoto.jsonl",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name:     "ref branch3",
			path:     "./testdata/dsse-branch3-ref-v1.intoto.jsonl",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name:     "invalid ref type",
			path:     "./testdata/dsse-invalid-ref-type-v1.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
		},
		{
			name:     "tag vSLSA1 case-sensitive",
			path:     "./testdata/dsse-vslsa1-tag.intoto.jsonl",
			tag:      "vSLSA1",
			expected: serrors.ErrorMismatchTag,
		},
		{
			name: "tag vslsa1",
			path: "./testdata/dsse-vslsa1-tag-v1.intoto.jsonl",
			tag:  "vslsa1",
		},
		{
			name: "case sensitive",
			path: "./testdata/dsse-vslsa1-tag-v1.intoto.jsonl",
			tag:  "vslsa1",
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.path)
			if err != nil {
				panic(fmt.Errorf("os.ReadFile: %w", err))
			}
			prov, err := provenanceFromBytes(content)
			if err != nil {
				panic(fmt.Errorf("provenanceFromBytes: %w", err))
			}

			err = VerifyTag(prov, tt.tag)
			if !errCmp(err, tt.expected) {
				t.Errorf(cmp.Diff(err, tt.expected))
			}
		})
	}
}

func Test_VerifyVersionedTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		tag      string
		expected error
	}{
		{
			name:     "ref main",
			path:     "./testdata/dsse-main-ref.intoto.jsonl",
			expected: serrors.ErrorInvalidSemver,
			tag:      "v1.2.3",
		},
		{
			name:     "ref branch3",
			path:     "./testdata/dsse-branch3-ref.intoto.jsonl",
			expected: serrors.ErrorInvalidSemver,
			tag:      "v1.2.3",
		},
		{
			name:     "tag v1.2 invalid versioning",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "1.2",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid ref",
			path:     "./testdata/dsse-invalid-ref-type.intoto.jsonl",
			expected: serrors.ErrorInvalidDssePayload,
			tag:      "v1.2.3",
		},
		{
			name:     "tag vslsa1 invalid",
			path:     "./testdata/dsse-vslsa1-tag.intoto.jsonl",
			tag:      "vslsa1",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "tag vslsa1 invalid semver",
			path:     "./testdata/dsse-vslsa1-tag.intoto.jsonl",
			tag:      "v1.2.3",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name: "tag v1.2.3 exact match",
			path: "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:  "v1.2.3",
		},
		{
			name: "tag v1.2.3 match v1.2",
			path: "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:  "v1.2",
		},
		{
			name: "tag v1.2.3 match v1",
			path: "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:  "v1",
		},
		{
			name:     "tag v1.2.3 no match v2",
			path:     "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:      "v2",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3 no match v1.3",
			path:     "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:      "v1.3",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3 no match v1.2.4",
			path:     "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:      "v1.2.4",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3 no match v1.2.2",
			path:     "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:      "v1.2.2",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1.2 exact v1.2",
			path: "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:  "v1.2",
		},
		{
			name: "tag v1.2 match v1",
			path: "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:  "v1",
		},
		{
			name:     "tag v1.1 no match v1.3",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.1",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v0 no match v1.3",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v0",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2 no match v1.3",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.3",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2 no match v1.2.3",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.2.3",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1.2 match v1.2.0",
			path: "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:  "v1.2.0",
		},
		{
			name: "tag v1.2 match v1.2.0+123",
			path: "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:  "v1.2.0+123",
		},
		{
			name:     "invalid v1.2+123",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.2+123",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid v1.2-alpha",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.2-alpha",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid v1-alpha",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1-alpha",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid v1+123",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1+123",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid v1-alpha+123",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1-alpha+123",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name:     "invalid v1.2-alpha+123",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.2-alpha+123",
			expected: serrors.ErrorInvalidSemver,
		},
		{
			name: "tag v1.2.3-alpha match v1.2.3-alpha",
			path: "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:  "v1.2.3-alpha",
		},
		{
			name:     "tag v1.2.3-alpha no match v1.2.3",
			path:     "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:      "v1.2.3",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1.2.3-alpha+123 match v1.2.3-alpha",
			path: "./testdata/dsse-v1.2.3-alpha+123-tag.intoto.jsonl",
			tag:  "v1.2.3-alpha",
		},
		{
			name: "tag v1.2.3-alpha+123 match v1.2.3-alpha+123",
			path: "./testdata/dsse-v1.2.3-alpha+123-tag.intoto.jsonl",
			tag:  "v1.2.3-alpha+123",
		},
		{
			name: "tag v1.2.3-alpha+123 match v1.2.3-alpha+456",
			path: "./testdata/dsse-v1.2.3-alpha+123-tag.intoto.jsonl",
			tag:  "v1.2.3-alpha+456",
		},
		{
			name: "tag v1.2.3-alpha match v1.2.3-alpha+123",
			path: "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:  "v1.2.3-alpha+123",
		},
		{
			name:     "tag v1.2.3-alpha no match v1.2.3-beta+123",
			path:     "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:      "v1.2.3-beta+123",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3+123 no match v1.2.3-alpha+123",
			path:     "./testdata/dsse-v1.2.3+123-tag.intoto.jsonl",
			tag:      "v1.2.3-alpha+123",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3+123 no match v1.2.3-alpha",
			path:     "./testdata/dsse-v1.2.3+123-tag.intoto.jsonl",
			tag:      "v1.2.3-alpha",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1.2.3+123 match v1.2.3+123",
			path: "./testdata/dsse-v1.2.3+123-tag.intoto.jsonl",
			tag:  "v1.2.3+123",
		},
		{
			name: "tag v1.2.3+123 match v1.2.3",
			path: "./testdata/dsse-v1.2.3+123-tag.intoto.jsonl",
			tag:  "v1.2.3",
		},
		{
			name: "tag v1.2.3+123 match v1.2.3+456",
			path: "./testdata/dsse-v1.2.3+123-tag.intoto.jsonl",
			tag:  "v1.2.3+456",
		},
		{
			name:     "tag v1.2.3 no match v1.2.3-aplha",
			path:     "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:      "v1.2.3-alpha",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2.3-alpha no match v1.2.3-beta",
			path:     "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:      "v1.2.3-beta",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2 no match v1.2.3-beta",
			path:     "./testdata/dsse-v1.2.3-alpha-tag.intoto.jsonl",
			tag:      "v1.2.3-beta",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1.2.3 match v1.2.3+123",
			path: "./testdata/dsse-v1.2.3-tag.intoto.jsonl",
			tag:  "v1.2.3+123",
		},
		{
			name:     "tag v1.2 no match v1.2.0-aplha+123",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v1.2.0-alpha+123",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1.2 no match v2",
			path:     "./testdata/dsse-v1.2-tag.intoto.jsonl",
			tag:      "v2",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1 exact match",
			path: "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:  "v1",
		},
		{
			name:     "tag v1 no match v2",
			path:     "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:      "v2",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1 no match v1.2",
			path:     "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:      "v1.2",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1 no match v0",
			path:     "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:      "v0",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name:     "tag v1 no match v1.2.3",
			path:     "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:      "v1.2.3",
			expected: serrors.ErrorMismatchVersionedTag,
		},
		{
			name: "tag v1 match v1.0",
			path: "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:  "v1.0",
		},
		{
			name: "tag v1 match v1.0.0",
			path: "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:  "v1.0.0",
		},
		{
			name:     "invalid v1-alpha",
			path:     "./testdata/dsse-v1-tag.intoto.jsonl",
			tag:      "v1-alpha",
			expected: serrors.ErrorInvalidSemver,
		},
	}
	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, version := range []string{"", "-v1"} {
				pathParts := strings.Split(tt.path, ".intoto")
				pathName := []string{pathParts[0] + version}
				pathName = append(pathName, pathParts[1:]...)
				content, err := os.ReadFile(strings.Join(pathName, ".intoto"))
				if err != nil {
					panic(fmt.Errorf("os.ReadFile: %w", err))
				}
				prov, err := provenanceFromBytes(content)
				if err != nil {
					panic(fmt.Errorf("provenanceFromBytes: %w", err))
				}

				err = VerifyVersionedTag(prov, tt.tag)
				if !errCmp(err, tt.expected) {
					t.Errorf(cmp.Diff(err, tt.expected))
				}
			}
		})
	}
}
