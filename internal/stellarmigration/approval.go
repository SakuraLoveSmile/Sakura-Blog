package stellarmigration

import (
	"fmt"
	"strconv"
	"strings"

	"blog/internal/post"
)

// MigrationApproval 是具名候选批准，不是任意目标库或生产执行授权。
type MigrationApproval struct {
	Approved        bool                  `json:"approved"`
	SourceDigest    string                `json:"source_digest"`
	CandidateDigest string                `json:"candidate_digest"`
	Resolutions     []MigrationResolution `json:"resolutions"`
	BodyDigests     map[string]string     `json:"body_digests,omitempty"`
}

type MigrationResolution struct {
	Source          string `json:"source"`
	Code            string `json:"code"`
	Path            string `json:"path"`
	Action          string `json:"action"`
	AlternativeName string `json:"alternative_name,omitempty"`
	ReplacementURL  string `json:"replacement_url,omitempty"`
}

type AppliedResolution struct {
	Original Diagnostic          `json:"original"`
	Decision MigrationResolution `json:"decision"`
}

func CandidateDigest(c Candidate) string             { return digest(c) }
func resolutionKey(source, code, path string) string { return source + "\x00" + code + "\x00" + path }

func resolveMigration(c Candidate, approval MigrationApproval, decisions NotebookStructureDecisions) (Candidate, NotebookStructurePreview, []AppliedResolution, error) {
	return resolveReviewedMigration(c, approval, decisions, false)
}

type ReviewedMigrationPreview struct {
	Candidate   Candidate                `json:"candidate"`
	Notebook    NotebookStructurePreview `json:"notebook"`
	Resolutions []AppliedResolution      `json:"resolutions"`
	BodyDigests map[string]string        `json:"body_digests"`
}

// PreviewReviewedSelections仅生成私密待审查候选，不创建DB/root，不授予正文或最终执行批准。
func PreviewReviewedSelections(c Candidate, resolutions []MigrationResolution, decisions NotebookStructureDecisions) (ReviewedMigrationPreview, error) {
	a := MigrationApproval{SourceDigest: c.Source.Digest, CandidateDigest: CandidateDigest(c), Resolutions: resolutions}
	resolved, nb, log, err := resolveReviewedMigration(c, a, decisions, true)
	if err != nil {
		return ReviewedMigrationPreview{}, err
	}
	bodies := map[string]string{}
	for _, p := range resolved.Records {
		original := ""
		if p.Table == "" {
			_, original, err = readLegacyDocument(c.Source.Files[p.Source])
			if err != nil {
				return ReviewedMigrationPreview{}, err
			}
		} else {
			for _, row := range c.Source.Tables[p.Table].Rows {
				if stringValue(row["id"]) == p.ID {
					original = body(row)
				}
			}
		}
		if original != p.Markdown {
			bodies[p.Source] = bytesDigest([]byte(p.Markdown))
		}
	}
	return ReviewedMigrationPreview{Candidate: resolved, Notebook: nb, Resolutions: log, BodyDigests: bodies}, nil
}

func resolveReviewedMigration(c Candidate, approval MigrationApproval, decisions NotebookStructureDecisions, previewOnly bool) (Candidate, NotebookStructurePreview, []AppliedResolution, error) {
	if (!previewOnly && !approval.Approved) || approval.SourceDigest != c.Source.Digest || approval.CandidateDigest != CandidateDigest(c) {
		return Candidate{}, NotebookStructurePreview{}, nil, fmt.Errorf("migration candidate/source approval missing or changed")
	}
	if c.Source.FormatVersion != SnapshotFormatVersion {
		return Candidate{}, NotebookStructurePreview{}, nil, fmt.Errorf("migration requires lossless snapshot format %d", SnapshotFormatVersion)
	}
	if err := validateAll(c); err != nil { // 待决schema仍允许进入具名修复，源完整性必须独立验证。
		if err := validateCandidateInventory(c); err != nil {
			return Candidate{}, NotebookStructurePreview{}, nil, err
		}
	}
	baseline, err := PreviewWithCapabilities(c.Source, c.ConsumerCapabilities)
	if err != nil || CandidateDigest(baseline) != CandidateDigest(c) {
		return Candidate{}, NotebookStructurePreview{}, nil, fmt.Errorf("reviewed candidate is not the complete original preview; diagnostics or values changed")
	}
	resolved := clone(c)
	nb, err := PreviewNotebookStructure(c.Source, decisions)
	if err != nil {
		return Candidate{}, nb, nil, err
	}
	if nb.Blocking() {
		return Candidate{}, nb, nil, fmt.Errorf("notebook structure decisions remain blocking")
	}
	resolutions := map[string]MigrationResolution{}
	for _, r := range approval.Resolutions {
		if (r.Action != "choose_alternative" && r.AlternativeName != "") || (r.Action != "replace_url" && r.ReplacementURL != "") {
			return Candidate{}, nb, nil, fmt.Errorf("resolution contains fields outside its action")
		}
		key := resolutionKey(r.Source, r.Code, r.Path)
		if _, exists := resolutions[key]; exists {
			return Candidate{}, nb, nil, fmt.Errorf("duplicate named migration resolution")
		}
		resolutions[key] = r
	}
	used := map[string]bool{}
	log := []AppliedResolution{}
	modified := map[string]string{}
	apply := func(d *Diagnostic) error {
		if !d.Blocking {
			return nil
		}
		key := resolutionKey(d.Source, d.Code, d.Path)
		r, exists := resolutions[key]
		if !exists || d.Kind == "consumer" {
			return fmt.Errorf("migration remains blocking: %s %s %s", d.Source, d.Code, d.Path)
		}
		if err := applyResolution(&resolved, *d, r, nb, modified); err != nil {
			return err
		}
		log = append(log, AppliedResolution{Original: clone(*d), Decision: r})
		d.Blocking = false
		d.Severity = "info"
		d.Message += "；待执行候选已应用绑定来源的具名选择，原诊断保留于resolution记录；不代表最终执行批准"
		used[key] = true
		return nil
	}
	for i := range resolved.Diagnostics {
		if err := apply(&resolved.Diagnostics[i]); err != nil {
			return Candidate{}, nb, nil, err
		}
	}
	for i := range resolved.Theme.Diagnostics {
		if err := apply(&resolved.Theme.Diagnostics[i]); err != nil {
			return Candidate{}, nb, nil, err
		}
	}
	for i := range resolved.Records {
		for j := range resolved.Records[i].Diagnostics {
			if err := apply(&resolved.Records[i].Diagnostics[j]); err != nil {
				return Candidate{}, nb, nil, err
			}
		}
	}
	if len(used) != len(resolutions) {
		return Candidate{}, nb, nil, fmt.Errorf("resolution does not match a blocking reviewed diagnostic")
	}
	// 结构转换重建canonical note元数据；宿主列写入仍来自nb批准预览，源快照不变。
	if c.Source.NotebookSchema.Required() {
		projected := clone(c.Source)
		for _, table := range []string{"notebooks", "notes"} {
			t := projected.Tables[table]
			t.Columns = notebookStructureColumns[table]
			t.Rows = nb.Rows[table]
			projected.Tables[table] = t
		}
		for i, p := range resolved.Records {
			if p.Table != "notebooks" && p.Table != "notes" {
				continue
			}
			for _, row := range nb.Rows[p.Table] {
				if stringValue(row["id"]) != p.ID {
					continue
				}
				var b *recordBuilder
				if p.Table == "notebooks" {
					b = buildCollection(projected, p.Table, row)
				} else {
					b = buildPage(projected, p.Table, row)
				}
				next := b.finish()
				next.SourceDigest = p.SourceDigest
				next.Diagnostics = p.Diagnostics
				next.Markdown = p.Markdown
				if p.Table == "notes" {
					next.Path = "/notebooks/" + stringValue(row["notebook_id"]) + "/" + stringValue(row["slug"])
				}
				resolved.Records[i] = next
			}
		}
	}
	if err := finishResolvedTheme(&resolved); err != nil {
		return Candidate{}, nb, nil, err
	}
	for _, p := range resolved.Records {
		original := ""
		if p.Table == "" {
			_, original, err = readLegacyDocument(c.Source.Files[p.Source])
			if err != nil {
				return Candidate{}, nb, nil, err
			}
		} else {
			for _, row := range c.Source.Tables[p.Table].Rows {
				if stringValue(row["id"]) == p.ID {
					original = body(row)
				}
			}
		}
		if !previewOnly && original != p.Markdown && approval.BodyDigests[p.Source] != bytesDigest([]byte(p.Markdown)) {
			return Candidate{}, nb, nil, fmt.Errorf("changed body needs explicit digest approval: %s", p.Source)
		}
	}
	for source, hash := range approval.BodyDigests {
		found := false
		for _, p := range resolved.Records {
			if p.Source == source && hash == bytesDigest([]byte(p.Markdown)) {
				found = true
			}
		}
		if !found {
			return Candidate{}, nb, nil, fmt.Errorf("unknown or changed approved body")
		}
	}
	if err := ValidateCandidate(resolved, c.Source); err != nil {
		return Candidate{}, nb, nil, err
	}
	return resolved, nb, log, nil
}

func validateCandidateInventory(c Candidate) error {
	// Preview的完整digest比对还会验证值；此处只避免schema修复路径掩盖缺记录。
	baseline, err := PreviewWithCapabilities(c.Source, c.ConsumerCapabilities)
	if err != nil {
		return err
	}
	if len(c.Records) != len(baseline.Records) {
		return fmt.Errorf("candidate record set incomplete")
	}
	return nil
}

func applyResolution(c *Candidate, d Diagnostic, r MigrationResolution, nb NotebookStructurePreview, modified map[string]string) error {
	record := func(source string) *RecordCandidate {
		for i := range c.Records {
			if c.Records[i].Source == source {
				return &c.Records[i]
			}
		}
		return nil
	}
	patch := func(p *RecordCandidate, path string, value any, blog bool) error {
		if p == nil {
			return fmt.Errorf("resolution target record missing")
		}
		key := p.Source + "/" + strconv.FormatBool(blog) + "/" + path
		if previous, exists := modified[key]; exists && previous != encode(value) {
			return fmt.Errorf("named resolutions conflict")
		}
		modified[key] = encode(value)
		text := p.ConfigJSON
		if blog {
			text = p.BlogJSON
		}
		raw, err := decode(text)
		if err != nil {
			return err
		}
		m := object(raw)
		if value == nil {
			deleteSEOPath(m, path)
		} else {
			set(m, path, value)
		}
		if blog {
			p.BlogJSON = encode(m)
		} else {
			p.ConfigJSON = encode(m)
		}
		return nil
	}
	switch d.Code {
	case "pjax_policy_removed", "data_cache_removed":
		if r.Action != "use_official" {
			return fmt.Errorf("unsupported runtime policy resolution")
		}
		return nil // 候选本身已为官方；明确批准替换旧策略，旧设置完整归档。
	case "legacy_notebook_schema", "note_missing_slug_status":
		if r.Action != "use_notebook_decisions" || nb.Blocking() {
			return fmt.Errorf("notebook conversion needs complete named metadata decisions")
		}
		return nil
	case "file_host_fields":
		if r.Action != "preserve_status" || d.Path != "status" {
			return fmt.Errorf("unsupported tracked host field resolution")
		}
		p := record(d.Source)
		if p == nil {
			return fmt.Errorf("tracked target missing")
		}
		source, _, err := readLegacyDocument(c.Source.Files[p.Source])
		if err != nil {
			return err
		}
		blog, err := decode(p.BlogJSON)
		if err != nil {
			return err
		}
		status, _ := get(object(blog), "status")
		if status != "draft" && status != "published" {
			return fmt.Errorf("tracked status invalid")
		}
		if encode(status) != encode(source["status"]) {
			return fmt.Errorf("tracked status changed")
		}
		parsed, err := post.ParseStellarDocument([]byte(p.Document), post.StellarDocumentOptions{Source: p.Source})
		if err != nil || encode(parsed.Blog["status"]) != encode(status) || parsed.Body != p.Markdown {
			return fmt.Errorf("tracked status/body roundtrip failed")
		}
		return nil
	case "wiki_name_title", "wiki_homepage", "banner_cover", "wiki_shelf_definition_conflict":
		if r.Action != "choose_alternative" {
			return fmt.Errorf("named alternative required")
		}
		var chosen *Alternative
		for i := range d.Alternatives {
			if d.Alternatives[i].Name == r.AlternativeName {
				chosen = &d.Alternatives[i]
			}
		}
		if chosen == nil {
			return fmt.Errorf("reviewed alternative missing")
		}
		target := d.Source
		if chosen.TargetSource != "" {
			target = chosen.TargetSource
		}
		p := record(target)
		raw, err := decode(chosen.ConfigJSON)
		if err != nil {
			return err
		}
		blog := map[string]any{}
		if chosen.BlogJSON != "" {
			value, err := decode(chosen.BlogJSON)
			if err != nil {
				return err
			}
			blog = object(value)
		}
		switch d.Code {
		case "wiki_name_title":
			return patch(p, "name", object(raw)["name"], false)
		case "wiki_shelf_definition_conflict":
			if r.AlternativeName == "保留当前项目表集合定义" {
				return nil
			}
			if !strings.HasSuffix(d.Path, ".name") && !strings.HasSuffix(d.Path, ".title") {
				return fmt.Errorf("shelf field has no supported canonical resolver")
			}
			return patch(p, "name", object(raw)["name"], false)
		case "wiki_homepage":
			value, _ := get(blog, "content.wiki_homepage_redirect")
			return patch(p, "content.wiki_homepage_redirect", value, true)
		case "banner_cover":
			if value, ok := get(blog, "content.banner_image"); ok {
				return patch(p, "content.banner_image", value, true)
			}
			if err := patch(p, "cover", object(raw)["cover"], false); err != nil {
				return err
			}
			value, _ := get(object(raw), "banner.background")
			if err := patch(p, "banner.background", value, false); err != nil {
				return err
			}
			return patch(p, "content.banner_image", nil, true)
		}
	case "invalid_value":
		if d.Source != "migration.theme" || !strings.HasPrefix(d.Path, "leftbar.footer.actions[") || !strings.HasSuffix(d.Path, "].url") {
			break
		}
		indexText := strings.TrimSuffix(strings.TrimPrefix(d.Path, "leftbar.footer.actions["), "].url")
		index, err := strconv.Atoi(indexText)
		if err != nil {
			return err
		}
		raw, err := decode(c.Theme.ThemeJSON)
		if err != nil {
			return err
		}
		m := object(raw)
		value, _ := get(m, "leftbar.footer.actions")
		actions := array(value)
		if index < 0 || index >= len(actions) || object(actions[index])["url"] != "#" {
			return fmt.Errorf("placeholder source changed")
		}
		switch r.Action {
		case "remove_placeholder":
			actions[index] = nil
		case "replace_url":
			object(actions[index])["url"] = r.ReplacementURL
		default:
			return fmt.Errorf("unsupported footer resolution")
		}
		set(m, "leftbar.footer.actions", actions)
		c.Theme.ThemeJSON = encode(m)
		return nil
	}
	return fmt.Errorf("unsupported blocking resolution: %s %s %s", d.Source, d.Code, d.Path)
}

func finishResolvedTheme(c *Candidate) error {
	raw, err := decode(c.Theme.ThemeJSON)
	if err != nil {
		return err
	}
	m := object(raw)
	if value, exists := get(m, "leftbar.footer.actions"); exists {
		actions := []any{}
		for _, v := range array(value) {
			if v != nil {
				actions = append(actions, v)
			}
		}
		set(m, "leftbar.footer.actions", actions)
	}
	c.Theme.ThemeJSON = encode(m)
	document, err := decode(c.Theme.DocumentJSON)
	if err != nil {
		return err
	}
	doc := object(document)
	doc["theme"] = m
	c.Theme.DocumentJSON = encode(doc)
	return nil
}
