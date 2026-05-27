package grappadcim

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	LayoutGridSchemaVersion    = "layout-grid-v1"
	LayoutGridImporterVersion  = "layout-grid-v1-importer-1"
	layoutGridSourceArtifact   = "artifacts/mappe/totali.json"
	layoutGridBlockKeyMaxBytes = 160
)

type LayoutGridImportOptions struct {
	SourcePath  string
	SourceLabel string
	DryRun      bool
	Now         time.Time
}

type LayoutGridImportReport struct {
	OK                  bool                         `json:"ok"`
	DryRun              bool                         `json:"dryRun"`
	Mode                string                       `json:"mode"`
	Source              string                       `json:"source"`
	StartedAt           string                       `json:"startedAt"`
	FinishedAt          string                       `json:"finishedAt"`
	ImporterVersion     string                       `json:"importerVersion"`
	SchemaVersion       string                       `json:"schemaVersion"`
	InputChecksum       string                       `json:"inputChecksum,omitempty"`
	Summary             LayoutGridImportSummary      `json:"summary"`
	Datacenters         []LayoutGridDatacenterReport `json:"datacenters"`
	Warnings            []string                     `json:"warnings,omitempty"`
	Error               string                       `json:"error,omitempty"`
	RecommendedExitCode int                          `json:"recommendedExitCode"`
}

type LayoutGridImportSummary struct {
	DatacentersInSource       int `json:"datacentersInSource"`
	DatacentersImported       int `json:"datacentersImported"`
	DatacentersUnresolved     int `json:"datacentersUnresolved"`
	BlocksInSource            int `json:"blocksInSource"`
	BlocksInserted            int `json:"blocksInserted"`
	BlocksUpdated             int `json:"blocksUpdated"`
	BlocksUnchanged           int `json:"blocksUnchanged"`
	BlocksWithUnresolvedIslet int `json:"blocksWithUnresolvedIslet"`
	PositionCells             int `json:"positionCells"`
	MissingPositionCells      int `json:"missingPositionCells"`
	PlenumCells               int `json:"plenumCells"`
	PlenumsLinked             int `json:"plenumsLinked"`
	PlenumsUnlinked           int `json:"plenumsUnlinked"`
	Warnings                  int `json:"warnings"`
}

type LayoutGridDatacenterReport struct {
	SourceName      string                  `json:"sourceName"`
	SourceType      string                  `json:"sourceType"`
	NormalizedType  string                  `json:"normalizedType,omitempty"`
	DatacenterID    *int                    `json:"datacenterId,omitempty"`
	DatacenterName  string                  `json:"datacenterName,omitempty"`
	Status          string                  `json:"status"`
	BlocksInSource  int                     `json:"blocksInSource"`
	BlocksProcessed int                     `json:"blocksProcessed"`
	Warnings        []string                `json:"warnings,omitempty"`
	Blocks          []LayoutGridBlockReport `json:"blocks,omitempty"`
}

type LayoutGridBlockReport struct {
	BlockKey         string                          `json:"blockKey"`
	Title            string                          `json:"title"`
	IsletName        string                          `json:"isletName"`
	DisplayOrder     int                             `json:"displayOrder"`
	LayoutWidth      string                          `json:"layoutWidth,omitempty"`
	Action           string                          `json:"action"`
	IsletID          *int                            `json:"isletId,omitempty"`
	SourceChecksum   string                          `json:"sourceChecksum"`
	PositionCells    int                             `json:"positionCells"`
	MissingPositions []LayoutGridMissingPosition     `json:"missingPositions,omitempty"`
	PlenumCells      int                             `json:"plenumCells"`
	LinkedPlenums    []LayoutGridPlenumBindingReport `json:"linkedPlenums,omitempty"`
	UnlinkedPlenums  []LayoutGridUnlinkedPlenum      `json:"unlinkedPlenums,omitempty"`
	Warnings         []string                        `json:"warnings,omitempty"`
}

type LayoutGridMissingPosition struct {
	RowIndex int `json:"rowIndex"`
	ColIndex int `json:"colIndex"`
	Pos      int `json:"pos"`
}

type LayoutGridPlenumBindingReport struct {
	RowIndex   int    `json:"rowIndex"`
	ColIndex   int    `json:"colIndex"`
	PlenumID   int    `json:"plenumId"`
	PlenumName string `json:"plenumName,omitempty"`
	PlenumType string `json:"plenumType,omitempty"`
	Label      string `json:"label,omitempty"`
}

type LayoutGridUnlinkedPlenum struct {
	RowIndex       int    `json:"rowIndex"`
	ColIndex       int    `json:"colIndex"`
	PlenumType     string `json:"plenumType,omitempty"`
	Reason         string `json:"reason"`
	CandidateCount int    `json:"candidateCount,omitempty"`
}

type layoutGridImportPlanBlock struct {
	DatacenterID           int
	DatacenterNameSnapshot string
	DatacenterKind         string
	IsletID                *int
	IsletNameSnapshot      string
	BlockKey               string
	BlockTitle             string
	DisplayOrder           int
	LayoutWidth            *string
	LayoutJSON             string
	SourceChecksum         string
	Action                 string
	PlenumBindings         []layoutGridPlenumBinding
}

type layoutGridPlenumBinding struct {
	DatacenterID int
	PlenumID     int
	RowIndex     int
	ColIndex     int
	PlenumType   string
	Label        string
}

type layoutSourceFile struct {
	Datacenters []layoutSourceDatacenter `json:"datacenters"`
}

type layoutSourceDatacenter struct {
	Name   string              `json:"name"`
	Type   string              `json:"type"`
	Blocks []layoutSourceBlock `json:"blocks"`
}

type layoutSourceBlock struct {
	IsletName   string             `json:"islet_name"`
	Title       string             `json:"title"`
	LayoutWidth string             `json:"layout_width"`
	Grid        [][]layoutGridCell `json:"grid"`
}

type layoutGridCell struct {
	Type       string `json:"type"`
	Pos        *int   `json:"pos,omitempty"`
	PlenumType string `json:"plenum_type,omitempty"`
	Text       string `json:"text,omitempty"`
}

type layoutGridPayload struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Source        layoutGridPayloadSource `json:"source"`
	Block         layoutGridPayloadBlock  `json:"block"`
	Grid          [][]layoutGridCell      `json:"grid"`
	RenderHints   layoutGridRenderHints   `json:"renderHints,omitempty"`
}

type layoutGridPayloadSource struct {
	Artifact         string `json:"artifact"`
	DatacenterName   string `json:"datacenterName"`
	DatacenterType   string `json:"datacenterType"`
	IsletName        string `json:"isletName"`
	SourceBlockIndex int    `json:"sourceBlockIndex"`
}

type layoutGridPayloadBlock struct {
	Title        string `json:"title"`
	LayoutWidth  string `json:"layoutWidth,omitempty"`
	DisplayOrder int    `json:"displayOrder"`
}

type layoutGridRenderHints struct {
	LayoutWidth string `json:"layoutWidth,omitempty"`
}

type layoutGridChecksumPayload struct {
	DatacenterName string             `json:"datacenterName"`
	DatacenterType string             `json:"datacenterType"`
	IsletName      string             `json:"isletName"`
	Title          string             `json:"title"`
	LayoutWidth    string             `json:"layoutWidth,omitempty"`
	DisplayOrder   int                `json:"displayOrder"`
	Grid           [][]layoutGridCell `json:"grid"`
}

type layoutDatacenterRef struct {
	ID   int
	Name string
}

type layoutIsletRef struct {
	ID           int
	DatacenterID int
	Name         string
}

type layoutPlenumRef struct {
	ID           int
	DatacenterID int
	Name         string
	Isle         string
	Type         string
	Status       string
}

type layoutExistingBlock struct {
	ID             int
	SourceChecksum string
	Active         bool
}

func RunLayoutGridImport(ctx context.Context, db *sql.DB, opts LayoutGridImportOptions) (LayoutGridImportReport, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sourcePath := strings.TrimSpace(opts.SourcePath)
	if sourcePath == "" {
		sourcePath = layoutGridSourceArtifact
	}
	sourceLabel := strings.TrimSpace(opts.SourceLabel)
	if sourceLabel == "" {
		sourceLabel = sourcePath
	}
	mode := "apply"
	if opts.DryRun {
		mode = "dry-run"
	}
	report := LayoutGridImportReport{
		OK:              true,
		DryRun:          opts.DryRun,
		Mode:            mode,
		Source:          sourceLabel,
		StartedAt:       now.Format(time.RFC3339),
		ImporterVersion: LayoutGridImporterVersion,
		SchemaVersion:   LayoutGridSchemaVersion,
		Datacenters:     []LayoutGridDatacenterReport{},
	}

	if db == nil {
		return failLayoutGridImportReport(report, errors.New("grappa database is required"))
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return failLayoutGridImportReport(report, fmt.Errorf("read source: %w", err))
	}
	report.InputChecksum = sha256Hex(raw)
	var source layoutSourceFile
	if err := json.Unmarshal(raw, &source); err != nil {
		return failLayoutGridImportReport(report, fmt.Errorf("decode source: %w", err))
	}
	if source.Datacenters == nil {
		return failLayoutGridImportReport(report, errors.New("source datacenters array is required"))
	}
	report.Summary.DatacentersInSource = len(source.Datacenters)

	if err := ensureLayoutGridImportSchema(ctx, db); err != nil {
		return failLayoutGridImportReport(report, err)
	}

	datacenters, err := loadLayoutDatacenters(ctx, db)
	if err != nil {
		return failLayoutGridImportReport(report, fmt.Errorf("load datacenters: %w", err))
	}

	planned := []layoutGridImportPlanBlock{}
	for dcIndex, sourceDC := range source.Datacenters {
		dcReport, blocks, err := buildLayoutGridDatacenterPlan(ctx, db, &report, datacenters, sourceDC, dcIndex)
		report.Datacenters = append(report.Datacenters, dcReport)
		planned = append(planned, blocks...)
		if err != nil {
			return failLayoutGridImportReport(report, err)
		}
	}

	if !opts.DryRun {
		if err := applyLayoutGridImportPlan(ctx, db, planned); err != nil {
			return failLayoutGridImportReport(report, fmt.Errorf("apply import: %w", err))
		}
	}

	finishLayoutGridImportReport(&report, nil)
	return report, nil
}

func buildLayoutGridDatacenterPlan(ctx context.Context, db *sql.DB, report *LayoutGridImportReport, datacenters []layoutDatacenterRef, sourceDC layoutSourceDatacenter, dcIndex int) (LayoutGridDatacenterReport, []layoutGridImportPlanBlock, error) {
	_ = dcIndex
	planned := []layoutGridImportPlanBlock{}
	sourceName := strings.TrimSpace(sourceDC.Name)
	sourceType := strings.TrimSpace(sourceDC.Type)
	dcReport := LayoutGridDatacenterReport{
		SourceName:     sourceName,
		SourceType:     sourceType,
		Status:         "pending",
		BlocksInSource: len(sourceDC.Blocks),
		Blocks:         []LayoutGridBlockReport{},
	}
	report.Summary.BlocksInSource += len(sourceDC.Blocks)

	kind, err := normalizeLayoutDatacenterType(sourceType)
	if err != nil {
		return dcReport, planned, fmt.Errorf("datacenter %q: %w", sourceName, err)
	}
	dcReport.NormalizedType = kind

	resolvedDC, status, candidates := resolveLayoutDatacenter(datacenters, sourceName)
	if status != "matched" {
		dcReport.Status = "skipped"
		report.Summary.DatacentersUnresolved++
		message := fmt.Sprintf("datacenter non risolto: %s", sourceName)
		if status == "ambiguous" {
			message = fmt.Sprintf("datacenter ambiguo: %s (%s)", sourceName, strings.Join(candidates, ", "))
		}
		addLayoutGridWarning(report, &dcReport, nil, message)
		return dcReport, planned, nil
	}
	dcReport.Status = "resolved"
	dcReport.DatacenterID = &resolvedDC.ID
	dcReport.DatacenterName = resolvedDC.Name
	report.Summary.DatacentersImported++

	islets, err := loadLayoutIslets(ctx, db, resolvedDC.ID)
	if err != nil {
		return dcReport, planned, fmt.Errorf("load islets for datacenter %q: %w", sourceName, err)
	}
	positionsByIslet, err := loadLayoutPositionNums(ctx, db, resolvedDC.ID)
	if err != nil {
		return dcReport, planned, fmt.Errorf("load positions for datacenter %q: %w", sourceName, err)
	}
	plenums, err := loadLayoutPlenums(ctx, db, resolvedDC.ID)
	if err != nil {
		return dcReport, planned, fmt.Errorf("load plenums for datacenter %q: %w", sourceName, err)
	}

	for blockIndex, sourceBlock := range sourceDC.Blocks {
		blockReport, blockPlan, err := buildLayoutGridBlockPlan(ctx, db, report, &dcReport, resolvedDC, kind, islets, positionsByIslet, plenums, sourceName, sourceBlock, blockIndex)
		dcReport.Blocks = append(dcReport.Blocks, blockReport)
		if blockPlan != nil {
			planned = append(planned, *blockPlan)
			dcReport.BlocksProcessed++
		}
		if err != nil {
			return dcReport, planned, err
		}
	}
	return dcReport, planned, nil
}

func buildLayoutGridBlockPlan(ctx context.Context, db *sql.DB, report *LayoutGridImportReport, dcReport *LayoutGridDatacenterReport, dc layoutDatacenterRef, kind string, islets []layoutIsletRef, positionsByIslet map[int]map[int]int, plenums []layoutPlenumRef, sourceDCName string, sourceBlock layoutSourceBlock, blockIndex int) (LayoutGridBlockReport, *layoutGridImportPlanBlock, error) {
	isletName := strings.TrimSpace(sourceBlock.IsletName)
	title := strings.TrimSpace(sourceBlock.Title)
	layoutWidth := strings.TrimSpace(sourceBlock.LayoutWidth)
	blockKey := layoutGridBlockKey(blockIndex, isletName, title)
	blockReport := LayoutGridBlockReport{
		BlockKey:        blockKey,
		Title:           title,
		IsletName:       isletName,
		DisplayOrder:    blockIndex,
		LayoutWidth:     layoutWidth,
		Action:          "pending",
		LinkedPlenums:   []LayoutGridPlenumBindingReport{},
		UnlinkedPlenums: []LayoutGridUnlinkedPlenum{},
	}

	if isletName == "" {
		return blockReport, nil, fmt.Errorf("datacenter %q block %d: islet_name required", sourceDCName, blockIndex)
	}
	if title == "" {
		return blockReport, nil, fmt.Errorf("datacenter %q block %d: title required", sourceDCName, blockIndex)
	}

	resolvedIslet, isletStatus, isletCandidates := resolveLayoutIslet(islets, isletName)
	var isletID *int
	if isletStatus == "matched" {
		isletID = &resolvedIslet.ID
		blockReport.IsletID = isletID
	} else {
		report.Summary.BlocksWithUnresolvedIslet++
		message := fmt.Sprintf("%s / %s: isola non risolta: %s", sourceDCName, title, isletName)
		if isletStatus == "ambiguous" {
			message = fmt.Sprintf("%s / %s: isola ambigua: %s (%s)", sourceDCName, title, isletName, strings.Join(isletCandidates, ", "))
		}
		addLayoutGridWarning(report, dcReport, &blockReport, message)
	}

	normalizedGrid, err := validateLayoutGridCells(report, dcReport, &blockReport, sourceDCName, title, sourceBlock.Grid)
	if err != nil {
		return blockReport, nil, err
	}

	if isletID != nil {
		knownNums := positionsByIslet[*isletID]
		for rowIndex, row := range normalizedGrid {
			for colIndex, cell := range row {
				if cell.Type != "position" || cell.Pos == nil {
					continue
				}
				if _, found := knownNums[*cell.Pos]; !found {
					missing := LayoutGridMissingPosition{RowIndex: rowIndex, ColIndex: colIndex, Pos: *cell.Pos}
					blockReport.MissingPositions = append(blockReport.MissingPositions, missing)
					report.Summary.MissingPositionCells++
					addLayoutGridWarning(report, dcReport, &blockReport, fmt.Sprintf("%s / %s: posizione %d non trovata nei dati Grappa", sourceDCName, title, *cell.Pos))
				}
			}
		}
	}

	plenumBindings := []layoutGridPlenumBinding{}
	for rowIndex, row := range normalizedGrid {
		for colIndex, cell := range row {
			if cell.Type != "plenum" {
				continue
			}
			blockReport.PlenumCells++
			report.Summary.PlenumCells++
			plenum, plenumStatus, candidateCount := resolveLayoutPlenum(plenums, isletName, cell.PlenumType)
			if plenumStatus != "matched" {
				reason := "missing"
				if plenumStatus == "ambiguous" {
					reason = "ambiguous"
				}
				unlinked := LayoutGridUnlinkedPlenum{RowIndex: rowIndex, ColIndex: colIndex, PlenumType: cell.PlenumType, Reason: reason, CandidateCount: candidateCount}
				blockReport.UnlinkedPlenums = append(blockReport.UnlinkedPlenums, unlinked)
				report.Summary.PlenumsUnlinked++
				addLayoutGridWarning(report, dcReport, &blockReport, fmt.Sprintf("%s / %s: plenum %s non collegato (%s)", sourceDCName, title, cell.PlenumType, reason))
				continue
			}
			label := layoutPlenumLabel(cell.PlenumType)
			binding := layoutGridPlenumBinding{DatacenterID: dc.ID, PlenumID: plenum.ID, RowIndex: rowIndex, ColIndex: colIndex, PlenumType: cell.PlenumType, Label: label}
			plenumBindings = append(plenumBindings, binding)
			blockReport.LinkedPlenums = append(blockReport.LinkedPlenums, LayoutGridPlenumBindingReport{RowIndex: rowIndex, ColIndex: colIndex, PlenumID: plenum.ID, PlenumName: plenum.Name, PlenumType: cell.PlenumType, Label: label})
			report.Summary.PlenumsLinked++
		}
	}

	payload := layoutGridPayload{
		SchemaVersion: LayoutGridSchemaVersion,
		Source: layoutGridPayloadSource{
			Artifact:         report.Source,
			DatacenterName:   sourceDCName,
			DatacenterType:   kind,
			IsletName:        isletName,
			SourceBlockIndex: blockIndex,
		},
		Block: layoutGridPayloadBlock{
			Title:        title,
			LayoutWidth:  layoutWidth,
			DisplayOrder: blockIndex,
		},
		Grid: normalizedGrid,
		RenderHints: layoutGridRenderHints{
			LayoutWidth: layoutWidth,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return blockReport, nil, fmt.Errorf("encode layout payload for %s / %s: %w", sourceDCName, title, err)
	}
	checksumBytes, err := json.Marshal(layoutGridChecksumPayload{
		DatacenterName: sourceDCName,
		DatacenterType: kind,
		IsletName:      isletName,
		Title:          title,
		LayoutWidth:    layoutWidth,
		DisplayOrder:   blockIndex,
		Grid:           normalizedGrid,
	})
	if err != nil {
		return blockReport, nil, fmt.Errorf("encode layout checksum payload for %s / %s: %w", sourceDCName, title, err)
	}
	checksum := sha256Hex(checksumBytes)
	blockReport.SourceChecksum = checksum

	existing, found, err := findLayoutExistingBlock(ctx, db, dc.ID, blockKey)
	if err != nil {
		return blockReport, nil, fmt.Errorf("check existing layout block %s / %s: %w", sourceDCName, title, err)
	}
	action := "insert"
	if found {
		action = "update"
		if existing.SourceChecksum == checksum && existing.Active {
			action = "unchanged"
		}
	}
	blockReport.Action = action
	switch action {
	case "insert":
		report.Summary.BlocksInserted++
	case "update":
		report.Summary.BlocksUpdated++
	case "unchanged":
		report.Summary.BlocksUnchanged++
	}

	blockReport.PositionCells = countLayoutCellType(normalizedGrid, "position")
	report.Summary.PositionCells += blockReport.PositionCells

	var layoutWidthPtr *string
	if layoutWidth != "" {
		layoutWidthPtr = &layoutWidth
	}
	plan := layoutGridImportPlanBlock{
		DatacenterID:           dc.ID,
		DatacenterNameSnapshot: dc.Name,
		DatacenterKind:         kind,
		IsletID:                isletID,
		IsletNameSnapshot:      isletName,
		BlockKey:               blockKey,
		BlockTitle:             title,
		DisplayOrder:           blockIndex,
		LayoutWidth:            layoutWidthPtr,
		LayoutJSON:             string(payloadBytes),
		SourceChecksum:         checksum,
		Action:                 action,
		PlenumBindings:         plenumBindings,
	}
	return blockReport, &plan, nil
}

func validateLayoutGridCells(report *LayoutGridImportReport, dcReport *LayoutGridDatacenterReport, blockReport *LayoutGridBlockReport, sourceDCName, title string, grid [][]layoutGridCell) ([][]layoutGridCell, error) {
	if grid == nil {
		return nil, fmt.Errorf("%s / %s: grid required", sourceDCName, title)
	}
	normalized := make([][]layoutGridCell, len(grid))
	for rowIndex, row := range grid {
		if row == nil {
			normalized[rowIndex] = []layoutGridCell{}
			continue
		}
		normalized[rowIndex] = make([]layoutGridCell, len(row))
		for colIndex, cell := range row {
			cellType := strings.TrimSpace(cell.Type)
			normalizedCell := layoutGridCell{Type: cellType}
			switch cellType {
			case "position":
				if cell.Pos == nil || *cell.Pos <= 0 {
					return nil, fmt.Errorf("%s / %s cell %d,%d: position requires positive pos", sourceDCName, title, rowIndex, colIndex)
				}
				pos := *cell.Pos
				normalizedCell.Pos = &pos
			case "empty":
				// No additional fields required.
			case "label":
				text := strings.TrimSpace(cell.Text)
				if text == "" {
					return nil, fmt.Errorf("%s / %s cell %d,%d: label requires text", sourceDCName, title, rowIndex, colIndex)
				}
				normalizedCell.Text = text
			case "plenum":
				plenumType := strings.TrimSpace(cell.PlenumType)
				if plenumType == "" {
					return nil, fmt.Errorf("%s / %s cell %d,%d: plenum requires plenum_type", sourceDCName, title, rowIndex, colIndex)
				}
				normalizedCell.PlenumType = plenumType
				if plenumType != "A" && plenumType != "B" {
					addLayoutGridWarning(report, dcReport, blockReport, fmt.Sprintf("%s / %s: plenum %s renderizzato come sconosciuto", sourceDCName, title, plenumType))
				}
			default:
				return nil, fmt.Errorf("%s / %s cell %d,%d: unsupported cell type %q", sourceDCName, title, rowIndex, colIndex, cellType)
			}
			normalized[rowIndex][colIndex] = normalizedCell
		}
	}
	return normalized, nil
}

func applyLayoutGridImportPlan(ctx context.Context, db *sql.DB, planned []layoutGridImportPlanBlock) error {
	return withTx(ctx, db, func(tx *sql.Tx) error {
		for _, block := range planned {
			blockID, err := upsertLayoutGridBlockTx(ctx, tx, block)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM dcim_layout_block_plenums WHERE layout_block_id = ?`, blockID); err != nil {
				return err
			}
			for _, binding := range block.PlenumBindings {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO dcim_layout_block_plenums
						(layout_block_id, datacenter_id, plenum_id, row_index, col_index, plenum_type, label)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					blockID,
					binding.DatacenterID,
					binding.PlenumID,
					binding.RowIndex,
					binding.ColIndex,
					nullIfBlank(binding.PlenumType),
					nullIfBlank(binding.Label),
				); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func upsertLayoutGridBlockTx(ctx context.Context, tx *sql.Tx, block layoutGridImportPlanBlock) (int, error) {
	existing, found, err := findLayoutExistingBlockTx(ctx, tx, block.DatacenterID, block.BlockKey)
	if err != nil {
		return 0, err
	}
	if !found {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO dcim_layout_blocks
				(datacenter_id, islet_id, datacenter_name_snapshot, datacenter_kind, islet_name_snapshot,
				 block_key, block_title, display_order, layout_width, schema_version, layout_json, source_checksum, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			block.DatacenterID,
			nullableIntPointer(block.IsletID),
			block.DatacenterNameSnapshot,
			block.DatacenterKind,
			block.IsletNameSnapshot,
			block.BlockKey,
			block.BlockTitle,
			block.DisplayOrder,
			nullableStringPointer(block.LayoutWidth),
			LayoutGridSchemaVersion,
			block.LayoutJSON,
			block.SourceChecksum,
		)
		if err != nil {
			return 0, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		return int(id), nil
	}
	if block.Action != "unchanged" || !existing.Active {
		if _, err := tx.ExecContext(ctx, `
			UPDATE dcim_layout_blocks
			SET islet_id = ?,
			    datacenter_name_snapshot = ?,
			    datacenter_kind = ?,
			    islet_name_snapshot = ?,
			    block_title = ?,
			    display_order = ?,
			    layout_width = ?,
			    schema_version = ?,
			    layout_json = ?,
			    source_checksum = ?,
			    active = 1
			WHERE id = ?`,
			nullableIntPointer(block.IsletID),
			block.DatacenterNameSnapshot,
			block.DatacenterKind,
			block.IsletNameSnapshot,
			block.BlockTitle,
			block.DisplayOrder,
			nullableStringPointer(block.LayoutWidth),
			LayoutGridSchemaVersion,
			block.LayoutJSON,
			block.SourceChecksum,
			existing.ID,
		); err != nil {
			return 0, err
		}
	}
	return existing.ID, nil
}

func ensureLayoutGridImportSchema(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN ('dcim_layout_blocks', 'dcim_layout_block_plenums')`)
	if err != nil {
		return fmt.Errorf("check layout grid schema: %w", err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("check layout grid schema scan: %w", err)
		}
		found[strings.ToLower(strings.TrimSpace(name))] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check layout grid schema rows: %w", err)
	}
	missing := []string{}
	for _, name := range []string{"dcim_layout_blocks", "dcim_layout_block_plenums"} {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("layout grid tables missing (%s): apply deploy/migrations/018_grappa_dcim_layout_grid.sql before running the importer", strings.Join(missing, ", "))
	}
	return nil
}

func loadLayoutDatacenters(ctx context.Context, db *sql.DB) ([]layoutDatacenterRef, error) {
	rows, err := db.QueryContext(ctx, `SELECT id_datacenter, name FROM datacenter ORDER BY id_datacenter ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []layoutDatacenterRef{}
	for rows.Next() {
		var item layoutDatacenterRef
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadLayoutIslets(ctx context.Context, db *sql.DB, datacenterID int) ([]layoutIsletRef, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, datacenter_id, name FROM islets WHERE datacenter_id = ? ORDER BY id ASC`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []layoutIsletRef{}
	for rows.Next() {
		var item layoutIsletRef
		if err := rows.Scan(&item.ID, &item.DatacenterID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadLayoutPositionNums(ctx context.Context, db *sql.DB, datacenterID int) (map[int]map[int]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.islets_id, p.num, MIN(p.id)
		FROM positions p
		JOIN islets i ON i.id = p.islets_id
		WHERE i.datacenter_id = ?
		GROUP BY p.islets_id, p.num`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]map[int]int{}
	for rows.Next() {
		var isletID, num, positionID int
		if err := rows.Scan(&isletID, &num, &positionID); err != nil {
			return nil, err
		}
		if result[isletID] == nil {
			result[isletID] = map[int]int{}
		}
		result[isletID][num] = positionID
	}
	return result, rows.Err()
}

func loadLayoutPlenums(ctx context.Context, db *sql.DB, datacenterID int) ([]layoutPlenumRef, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, datacenter_id, name, isle, type, status FROM plenums WHERE datacenter_id = ? ORDER BY id ASC`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []layoutPlenumRef{}
	for rows.Next() {
		var item layoutPlenumRef
		var name, isle, plenumType, status sql.NullString
		if err := rows.Scan(&item.ID, &item.DatacenterID, &name, &isle, &plenumType, &status); err != nil {
			return nil, err
		}
		item.Name = strings.TrimSpace(name.String)
		item.Isle = strings.TrimSpace(isle.String)
		item.Type = strings.TrimSpace(plenumType.String)
		item.Status = strings.TrimSpace(status.String)
		items = append(items, item)
	}
	return items, rows.Err()
}

func findLayoutExistingBlock(ctx context.Context, db *sql.DB, datacenterID int, blockKey string) (layoutExistingBlock, bool, error) {
	row := db.QueryRowContext(ctx, `SELECT id, source_checksum, active FROM dcim_layout_blocks WHERE datacenter_id = ? AND block_key = ?`, datacenterID, blockKey)
	return scanLayoutExistingBlock(row)
}

func findLayoutExistingBlockTx(ctx context.Context, tx *sql.Tx, datacenterID int, blockKey string) (layoutExistingBlock, bool, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, source_checksum, active FROM dcim_layout_blocks WHERE datacenter_id = ? AND block_key = ? FOR UPDATE`, datacenterID, blockKey)
	return scanLayoutExistingBlock(row)
}

type layoutExistingBlockScanner interface {
	Scan(dest ...any) error
}

func scanLayoutExistingBlock(row layoutExistingBlockScanner) (layoutExistingBlock, bool, error) {
	var item layoutExistingBlock
	var checksum sql.NullString
	var active int
	if err := row.Scan(&item.ID, &checksum, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, false, nil
		}
		return item, false, err
	}
	if checksum.Valid {
		item.SourceChecksum = strings.TrimSpace(checksum.String)
	}
	item.Active = active == 1
	return item, true, nil
}

func normalizeLayoutDatacenterType(sourceType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "dc":
		return "room", nil
	case "mmr":
		return "mmr", nil
	default:
		return "", fmt.Errorf("unsupported datacenter type %q", sourceType)
	}
}

func resolveLayoutDatacenter(items []layoutDatacenterRef, name string) (layoutDatacenterRef, string, []string) {
	exact := []layoutDatacenterRef{}
	folded := []layoutDatacenterRef{}
	compact := []layoutDatacenterRef{}
	target := strings.TrimSpace(name)
	targetCompact := compactLayoutLookupKey(target)
	for _, item := range items {
		candidate := strings.TrimSpace(item.Name)
		if candidate == target {
			exact = append(exact, item)
		}
		if strings.EqualFold(candidate, target) {
			folded = append(folded, item)
		}
		if compactLayoutLookupKey(candidate) == targetCompact {
			compact = append(compact, item)
		}
	}
	if len(exact) == 1 {
		return exact[0], "matched", nil
	}
	if len(exact) > 1 {
		return layoutDatacenterRef{}, "ambiguous", layoutDatacenterCandidateLabels(exact)
	}
	if len(folded) == 1 {
		return folded[0], "matched", nil
	}
	if len(folded) > 1 {
		return layoutDatacenterRef{}, "ambiguous", layoutDatacenterCandidateLabels(folded)
	}
	if len(compact) == 1 {
		return compact[0], "matched", nil
	}
	if len(compact) > 1 {
		return layoutDatacenterRef{}, "ambiguous", layoutDatacenterCandidateLabels(compact)
	}
	return layoutDatacenterRef{}, "missing", nil
}

func resolveLayoutIslet(items []layoutIsletRef, name string) (layoutIsletRef, string, []string) {
	exact := []layoutIsletRef{}
	folded := []layoutIsletRef{}
	compact := []layoutIsletRef{}
	target := strings.TrimSpace(name)
	targetCompact := compactLayoutLookupKey(target)
	for _, item := range items {
		candidate := strings.TrimSpace(item.Name)
		if candidate == target {
			exact = append(exact, item)
		}
		if strings.EqualFold(candidate, target) {
			folded = append(folded, item)
		}
		if compactLayoutLookupKey(candidate) == targetCompact {
			compact = append(compact, item)
		}
	}
	if len(exact) == 1 {
		return exact[0], "matched", nil
	}
	if len(exact) > 1 {
		return layoutIsletRef{}, "ambiguous", layoutIsletCandidateLabels(exact)
	}
	if len(folded) == 1 {
		return folded[0], "matched", nil
	}
	if len(folded) > 1 {
		return layoutIsletRef{}, "ambiguous", layoutIsletCandidateLabels(folded)
	}
	if len(compact) == 1 {
		return compact[0], "matched", nil
	}
	if len(compact) > 1 {
		return layoutIsletRef{}, "ambiguous", layoutIsletCandidateLabels(compact)
	}
	return layoutIsletRef{}, "missing", nil
}

func resolveLayoutPlenum(items []layoutPlenumRef, isletName, plenumType string) (layoutPlenumRef, string, int) {
	targetIslet := strings.TrimSpace(isletName)
	targetType := strings.TrimSpace(plenumType)
	targetIsletCompact := compactLayoutLookupKey(targetIslet)
	targetTypeCompact := compactLayoutLookupKey(targetType)
	exact := []layoutPlenumRef{}
	folded := []layoutPlenumRef{}
	compact := []layoutPlenumRef{}
	for _, item := range items {
		candidateIsle := strings.TrimSpace(item.Isle)
		candidateType := strings.TrimSpace(item.Type)
		if candidateIsle == targetIslet && candidateType == targetType {
			exact = append(exact, item)
		}
		if strings.EqualFold(candidateIsle, targetIslet) && strings.EqualFold(candidateType, targetType) {
			folded = append(folded, item)
		}
		if compactLayoutLookupKey(candidateIsle) == targetIsletCompact && compactLayoutLookupKey(candidateType) == targetTypeCompact {
			compact = append(compact, item)
		}
	}
	if len(exact) == 1 {
		return exact[0], "matched", 1
	}
	if len(exact) > 1 {
		return layoutPlenumRef{}, "ambiguous", len(exact)
	}
	if len(folded) == 1 {
		return folded[0], "matched", 1
	}
	if len(folded) > 1 {
		return layoutPlenumRef{}, "ambiguous", len(folded)
	}
	if len(compact) == 1 {
		return compact[0], "matched", 1
	}
	if len(compact) > 1 {
		return layoutPlenumRef{}, "ambiguous", len(compact)
	}
	return layoutPlenumRef{}, "missing", 0
}

func layoutDatacenterCandidateLabels(items []layoutDatacenterRef) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, fmt.Sprintf("%d:%s", item.ID, strings.TrimSpace(item.Name)))
	}
	sort.Strings(labels)
	return labels
}

func layoutIsletCandidateLabels(items []layoutIsletRef) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, fmt.Sprintf("%d:%s", item.ID, strings.TrimSpace(item.Name)))
	}
	sort.Strings(labels)
	return labels
}

func addLayoutGridWarning(report *LayoutGridImportReport, dcReport *LayoutGridDatacenterReport, blockReport *LayoutGridBlockReport, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	report.Warnings = append(report.Warnings, message)
	if dcReport != nil {
		dcReport.Warnings = append(dcReport.Warnings, message)
	}
	if blockReport != nil {
		blockReport.Warnings = append(blockReport.Warnings, message)
	}
}

func finishLayoutGridImportReport(report *LayoutGridImportReport, err error) {
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	report.Summary.Warnings = len(report.Warnings)
	if err != nil {
		report.OK = false
		report.Error = err.Error()
		report.RecommendedExitCode = 1
		return
	}
	report.OK = true
	if len(report.Warnings) > 0 {
		report.RecommendedExitCode = 2
	} else {
		report.RecommendedExitCode = 0
	}
}

func failLayoutGridImportReport(report LayoutGridImportReport, err error) (LayoutGridImportReport, error) {
	finishLayoutGridImportReport(&report, err)
	return report, err
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func layoutGridBlockKey(displayOrder int, isletName, title string) string {
	key := fmt.Sprintf("grid:%d:%s:%s", displayOrder, slugLayoutGridPart(isletName), slugLayoutGridPart(title))
	return trimLayoutGridKey(key, layoutGridBlockKeyMaxBytes)
}

func trimLayoutGridKey(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	cut := 0
	for index := range value {
		if index > maxBytes {
			break
		}
		cut = index
	}
	if cut <= 0 {
		cut = maxBytes
	}
	return strings.TrimRight(value[:cut], "-")
}

func compactLayoutLookupKey(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func slugLayoutGridPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "na"
	}
	return result
}

func countLayoutCellType(grid [][]layoutGridCell, cellType string) int {
	count := 0
	for _, row := range grid {
		for _, cell := range row {
			if cell.Type == cellType {
				count++
			}
		}
	}
	return count
}

func layoutPlenumLabel(plenumType string) string {
	plenumType = strings.TrimSpace(plenumType)
	if plenumType == "" {
		return "Plenum"
	}
	return "Plenum " + plenumType
}

func nullableIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStringPointer(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func nullIfBlank(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
