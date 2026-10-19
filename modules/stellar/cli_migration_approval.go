package stellar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"

	"blog/internal/stellarmigration"
)

// 由宿主明确声明当前实现的能力；独立迁移包的 nil/空能力默认仍然阻断。
func migrationHostCapabilities() stellarmigration.ConsumerCapabilities {
	return stellarmigration.ConsumerCapabilities{
		stellarmigration.ConsumerPoster: true, stellarmigration.ConsumerCollectionOrder: true,
		stellarmigration.ConsumerAuthors: true, stellarmigration.ConsumerAuthorWidget: true,
		stellarmigration.ConsumerWikiExternalURL: true, stellarmigration.ConsumerAvatarBackground: true,
		stellarmigration.ConsumerRightbarIntro: true, stellarmigration.ConsumerWelcomeLinklist: true,
		stellarmigration.ConsumerNavTabs: true, stellarmigration.ConsumerAILabel: true,
		stellarmigration.ConsumerCategoryIcons: true, stellarmigration.ConsumerTagIcons: true,
		stellarmigration.ConsumerReadNext: true, stellarmigration.ConsumerExpireDays: true,
		stellarmigration.ConsumerBannerRatio: true, stellarmigration.ConsumerEditThisPage: true,
		stellarmigration.ConsumerNotebookLicense: true, stellarmigration.ConsumerNotebookShare: true,
		stellarmigration.ConsumerPages: true, stellarmigration.ConsumerSEO: true,
	}
}

type migrationDecisionGuide struct {
	Resolutions  []migrationResolutionGuide    `json:"resolutions"`
	Notebook     []stellarmigration.Diagnostic `json:"notebook_required_fields"`
	Instructions []string                      `json:"instructions"`
}
type migrationResolutionGuide struct {
	Source       string   `json:"source"`
	Code         string   `json:"code"`
	Path         string   `json:"path"`
	Actions      []string `json:"allowed_actions"`
	Alternatives []string `json:"alternative_names,omitempty"`
}

func migrationSelectionNextSteps() []string {
	return []string{
		"Read report.public.json and decisions-guide.private.json; edit a private copy of approval-template.private.json. Leave approval.approved=false; explicitly fill each supported resolution and required Notebook field.",
		"Run stellar migrate preview --review <this-directory> --approval <edited-private-template> --out <new-selected-review-directory>.",
		"Unknown/schema/consumer blockers require correcting the source or implementation, then a new preview; do not remove diagnostics or invent an action.",
	}
}

func migrationDecisionTemplate(capture stellarmigration.RootCapture, c stellarmigration.Candidate) (migrationCLIApproval, migrationDecisionGuide) {
	pending := migrationCLIApproval{
		Approval: stellarmigration.RootMigrationApproval{MigrationApproval: stellarmigration.MigrationApproval{SourceDigest: capture.Source.Digest, CandidateDigest: stellarmigration.CandidateDigest(c), Resolutions: []stellarmigration.MigrationResolution{}}, RootDigest: capture.Digest},
		Notebook: stellarmigration.NotebookStructureDecisions{SourceDigest: capture.Source.Digest, Records: map[string]stellarmigration.NotebookRecordMetadata{}},
		Offline:  migrationOfflineAttestation{RootDigest: capture.Digest},
	}
	guide := migrationDecisionGuide{Resolutions: []migrationResolutionGuide{}, Notebook: []stellarmigration.Diagnostic{}, Instructions: migrationSelectionNextSteps()}
	add := func(d stellarmigration.Diagnostic) {
		if !d.Blocking {
			return
		}
		item := migrationResolutionGuide{Source: d.Source, Code: d.Code, Path: d.Path, Actions: []string{}}
		switch d.Code {
		case "site_url_required":
			item.Actions = []string{"replace_url"}
		case "pjax_policy_removed", "data_cache_removed":
			item.Actions = []string{"use_official"}
		case "legacy_notebook_schema", "note_missing_slug_status":
			item.Actions = []string{"use_notebook_decisions"}
		case "file_host_fields":
			if d.Path == "status" {
				item.Actions = []string{"preserve_status"}
			}
		case "wiki_name_title", "wiki_homepage", "banner_cover", "wiki_shelf_definition_conflict":
			item.Actions = []string{"choose_alternative"}
			for _, alternative := range d.Alternatives {
				item.Alternatives = append(item.Alternatives, alternative.Name)
			}
		case "invalid_value":
			if d.Source == "migration.theme" && strings.HasPrefix(d.Path, "leftbar.footer.actions[") && strings.HasSuffix(d.Path, "].url") {
				item.Actions = []string{"remove_placeholder", "replace_url"}
			}
		}
		guide.Resolutions = append(guide.Resolutions, item)
		if len(item.Actions) > 0 {
			pending.Approval.Resolutions = append(pending.Approval.Resolutions, stellarmigration.MigrationResolution{Source: d.Source, Code: d.Code, Path: d.Path})
		}
	}
	for _, d := range c.Diagnostics {
		add(d)
	}
	for _, d := range c.Theme.Diagnostics {
		add(d)
	}
	for _, p := range c.Records {
		for _, d := range p.Diagnostics {
			add(d)
		}
	}
	if nb, err := stellarmigration.PreviewNotebookStructure(capture.Source, pending.Notebook); err == nil {
		for _, d := range nb.Diagnostics {
			if !d.Blocking {
				continue
			}
			guide.Notebook = append(guide.Notebook, d)
			if d.Code == "notebook_metadata_required" {
				pending.Notebook.Records[d.Source] = stellarmigration.NotebookRecordMetadata{}
			}
		}
	}
	return pending, guide
}

type migrationApprovalError struct {
	code  string
	cause error
}

func (e *migrationApprovalError) Error() string { return e.code }
func (e *migrationApprovalError) Unwrap() error { return e.cause }
func migrationApprovalFailure(code string, cause error) error {
	return &migrationApprovalError{code, cause}
}
func migrationApprovalErrorCode(err error) string {
	var failure *migrationApprovalError
	if errors.As(err, &failure) {
		return failure.code
	}
	return "MIGRATION_REJECTED"
}
func migrationErrorHint(code string) string {
	switch code {
	case "CONFIG_INVALID":
		return "cannot resolve runtime configuration; check -root, -config and BLOG_* overrides (values withheld)"
	case "BINARY_UNAVAILABLE":
		return "cannot identify the current executable; use a built Blog binary"
	case "ARTIFACT_INVALID":
		return "review files must be valid JSON in ordinary private directories (0700) with private files (0600); regenerate missing or modified artifacts"
	case "REVIEW_CHANGED":
		return "review artifacts or selections changed; generate and inspect a new selected preview before approval"
	case "SOURCE_CHANGED":
		return "runtime files, database or source inventory changed since preview; stop writers and generate a new preview"
	case "LAYOUT_CHANGED":
		return "review belongs to different runtime resources; use the original -root/-config and environment"
	case "SELECTIONS_BLOCKED":
		return "selected resolutions or Notebook metadata remain invalid/blocking; complete the private decisions and regenerate selected preview"
	case "BODY_APPROVAL_REQUIRED":
		return "inspect selected-candidate.private.json and body-review.public.json, then pass --accept-body-changes to approve those exact bodies"
	case "OUTPUT_INVALID":
		return "--out must name a new directory outside runtime resources with an existing parent; existing files are never overwritten"
	case "INVENTORY_INVALID":
		return "check content manifest and Markdown paths; tracked files must be ordinary files inside content/"
	case "CAPABILITIES_INVALID":
		return "check private --capabilities JSON against the supported capability names; an explicit empty/null map disables all capabilities"
	case "CAPTURE_FAILED":
		return "cannot capture complete runtime resources; check database readability, ordinary resource paths and source consistency"
	case "PREVIEW_FAILED":
		return "source cannot produce a valid migration preview; check source schema/configuration privately and correct it before retrying"
	case "REHEARSAL_FAILED":
		return "check digest-bound approval, Notebook decisions and body approval; generate a fresh selected preview if the source changed"
	case "RUNTIME_VALIDATION_FAILED":
		return "staged Bootstrap/search/SSR validation failed; inspect the rehearsal's private validation artifact before retrying"
	case "APPLY_FAILED":
		return "offline apply rejected; inspect approval, offline prerequisites and any pending recovery journal before retrying"
	case "RECOVERY_FAILED":
		return "offline recovery rejected; preserve the migration journal and backup and check runtime resources before retrying"
	default:
		return "check private review artifacts, source consistency, digest-bound approval and runtime prerequisites; no private values are printed"
	}
}

// 批准重新读取真实来源并逐份重建审核产物；public report和正文摘要均不是信任来源。
func approveMigrationSelections(ctx context.Context, layout stellarmigration.RootLayout, review, requested string, acceptBody, offline bool) (migrationCLISummary, error) {
	fail := func(code string, err error) (migrationCLISummary, error) {
		return migrationCLISummary{}, migrationApprovalFailure(code, err)
	}
	var capture stellarmigration.RootCapture
	var candidate stellarmigration.Candidate
	var pending migrationCLIApproval
	for _, artifact := range []struct {
		name  string
		value any
	}{
		{"root-capture.private.json", &capture}, {"candidate.private.json", &candidate}, {"approval-template.private.json", &pending},
	} {
		if err := readMigrationPrivateJSON(filepath.Join(review, artifact.name), artifact.value); err != nil {
			return fail("ARTIFACT_INVALID", err)
		}
	}
	if err := capture.Verify(); err != nil {
		return fail("REVIEW_CHANGED", err)
	}
	if !sameMigrationLayout(capture.Layout, layout) {
		return fail("LAYOUT_CHANGED", nil)
	}
	if pending.Approval.Approved || pending.Offline.Approved || len(pending.Approval.BodyDigests) != 0 || pending.Approval.RootDigest != capture.Digest || pending.Approval.SourceDigest != capture.Source.Digest || pending.Approval.CandidateDigest != stellarmigration.CandidateDigest(candidate) {
		return fail("REVIEW_CHANGED", nil)
	}
	inventory, err := migrationSourceInventory(layout.Root, capture.TrackedSources)
	if err != nil || !reflect.DeepEqual(inventory, capture.TrackedSources) {
		return fail("SOURCE_CHANGED", err)
	}
	fresh, err := stellarmigration.CaptureMigrationRoot(ctx, layout, inventory)
	if err != nil {
		return fail("CAPTURE_FAILED", err)
	}
	if fresh.Digest != capture.Digest {
		return fail("SOURCE_CHANGED", nil)
	}
	baseline, err := stellarmigration.PreviewWithCapabilities(fresh.Source, candidate.ConsumerCapabilities)
	if err != nil || stellarmigration.CandidateDigest(baseline) != stellarmigration.CandidateDigest(candidate) {
		return fail("REVIEW_CHANGED", err)
	}
	selected, err := stellarmigration.PreviewReviewedSelections(candidate, pending.Approval.Resolutions, pending.Notebook)
	if err != nil || selected.Candidate.PublicReport().Blocking != 0 {
		return fail("SELECTIONS_BLOCKED", err)
	}
	hash, err := stellarmigration.RootReviewDigest(fresh, baseline, pending.Approval.Resolutions, pending.Notebook)
	if err != nil || hash != pending.Approval.ReviewDigest || hash == "" {
		return fail("REVIEW_CHANGED", err)
	}
	// 不只校验摘要：拒绝用户看过的任何选值、正文或公开审核文件被单独替换。
	var storedSelected stellarmigration.Candidate
	var storedReview stellarmigration.ReviewedMigrationPreview
	var originalReport, report stellarmigration.Report
	var bodies map[string]string
	for _, artifact := range []struct {
		name     string
		value    any
		expected any
	}{
		{"selected-candidate.private.json", &storedSelected, selected.Candidate},
		{"reviewed-selections.private.json", &storedReview, selected},
		{"original-report.public.json", &originalReport, candidate.PublicReport()},
		{"report.public.json", &report, selected.Candidate.PublicReport()},
		{"body-review.public.json", &bodies, selected.BodyDigests},
	} {
		if err := readMigrationPrivateJSON(filepath.Join(review, artifact.name), artifact.value); err != nil {
			return fail("ARTIFACT_INVALID", err)
		}
		if !migrationJSONEqual(artifact.value, artifact.expected) {
			return fail("REVIEW_CHANGED", nil)
		}
	}
	if len(selected.BodyDigests) > 0 && !acceptBody {
		return fail("BODY_APPROVAL_REQUIRED", nil)
	}
	pending.Approval.Approved = true
	pending.Approval.BodyDigests = selected.BodyDigests
	pending.Approval.RootDigest = fresh.Digest
	pending.Approval.SourceDigest = fresh.Source.Digest
	pending.Approval.CandidateDigest = stellarmigration.CandidateDigest(baseline)
	pending.Approval.ReviewDigest = hash
	pending.Offline = migrationOfflineAttestation{Approved: offline, RootDigest: fresh.Digest, StoppedServerAndWorkers: offline, LegacyInstancesChecked: offline}
	dir, err := createMigrationPreviewDirectory(requested, layout)
	if err != nil {
		return fail("OUTPUT_INVALID", err)
	}
	path := filepath.Join(dir, "approval.private.json")
	if err = writeMigrationPrivateJSON(path, pending); err != nil {
		return fail("OUTPUT_INVALID", err)
	}
	steps := []string{"Run stellar migrate rehearse --review <selected-review-directory> --approval <approval_file>; inspect the staged runtime validation before apply."}
	if offline {
		steps = append(steps, "After a successful rehearsal, explicitly run stellar migrate apply --review <selected-review-directory> --approval <approval_file>; approval itself does not apply changes.")
	} else {
		steps = append(steps, "Apply remains blocked. Stop every server/worker and check legacy instances; rerun approve into a new directory with --server-stopped --legacy-instances-checked (and --accept-body-changes when needed).")
	}
	return migrationCLISummary{OK: true, Operation: "approve", Directory: dir, ApprovalFile: path, OfflineApproved: offline, RootDigest: fresh.Digest, SourceDigest: fresh.Source.Digest, CandidateDigest: stellarmigration.CandidateDigest(baseline), SelectedCandidateDigest: stellarmigration.CandidateDigest(selected.Candidate), ReviewDigest: hash, NextSteps: steps}, nil
}

func migrationJSONEqual(a, b any) bool {
	left, le := json.Marshal(a)
	right, re := json.Marshal(b)
	return le == nil && re == nil && bytes.Equal(left, right)
}
