package grappadcim

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type layoutGridBlockBuild struct {
	block      LayoutGridBlock
	sourceGrid [][]layoutGridCell
}

type layoutGridCellKey struct {
	blockID  int
	rowIndex int
	colIndex int
}

type layoutGridPositionKey struct {
	isletID int
	num     int
}

type layoutGridPlenumLiveBinding struct {
	plenumID     int
	plenumName   *string
	plenumStatus *string
	plenumType   *string
}

func (h *Handler) handleGetDatacenterLayoutGrid(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	datacenterID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	dc, found, err := h.getDatacenter(r, datacenterID)
	if err != nil {
		h.dbFailure(w, r, "layout_grid_datacenter", err, "datacenter_id", datacenterID)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "datacenter_not_found")
		return
	}
	positions, err := h.listPositionsForDatacenter(r, datacenterID)
	if err != nil {
		h.dbFailure(w, r, "layout_grid_positions", err, "datacenter_id", datacenterID)
		return
	}
	racks, err := h.listRacksForDatacenter(r, datacenterID)
	if err != nil {
		h.dbFailure(w, r, "layout_grid_racks", err, "datacenter_id", datacenterID)
		return
	}
	blocks, warnings, err := h.listLayoutGridBlocks(r, dc, positions)
	if err != nil {
		httputil.InternalError(w, r, err, "layout grid read failed", "component", component, "operation", "layout_grid_blocks", "datacenter_id", datacenterID)
		return
	}
	incomplete := len(blocks) == 0 || len(warnings) > 0
	if len(blocks) == 0 {
		warnings = append(warnings, "Mappa non configurata")
	}
	httputil.JSON(w, http.StatusOK, LayoutGridResponse{
		Datacenter: dc,
		Blocks:     blocks,
		Positions:  positions,
		Racks:      racks,
		Incomplete: incomplete,
		Warnings:   warnings,
	})
}

func (h *Handler) listLayoutGridBlocks(r *http.Request, dc Datacenter, positions []Position) ([]LayoutGridBlock, []string, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT id, datacenter_id, islet_id, islet_name_snapshot, block_title, layout_width,
		       display_order, schema_version, layout_json
		FROM dcim_layout_blocks
		WHERE datacenter_id = ? AND active = 1
		ORDER BY display_order ASC, id ASC`, dc.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	builds := []layoutGridBlockBuild{}
	blockIDs := []int{}
	warnings := []string{}
	for rows.Next() {
		var blockID, datacenterID, displayOrder int
		var isletID sql.NullInt64
		var isletName, title, layoutWidth, schemaVersion, layoutJSON sql.NullString
		if err := rows.Scan(&blockID, &datacenterID, &isletID, &isletName, &title, &layoutWidth, &displayOrder, &schemaVersion, &layoutJSON); err != nil {
			return nil, nil, err
		}
		var payload layoutGridPayload
		if err := json.Unmarshal([]byte(layoutJSON.String), &payload); err != nil {
			return nil, nil, fmt.Errorf("decode layout block %d: %w", blockID, err)
		}
		effectiveSchemaVersion := strings.TrimSpace(schemaVersion.String)
		if effectiveSchemaVersion == "" {
			effectiveSchemaVersion = strings.TrimSpace(payload.SchemaVersion)
		}
		if effectiveSchemaVersion != LayoutGridSchemaVersion {
			return nil, nil, fmt.Errorf("layout block %d has unsupported schema version %q", blockID, effectiveSchemaVersion)
		}
		effectiveIsletName := strings.TrimSpace(isletName.String)
		if effectiveIsletName == "" {
			effectiveIsletName = strings.TrimSpace(payload.Source.IsletName)
		}
		effectiveTitle := strings.TrimSpace(title.String)
		if effectiveTitle == "" {
			effectiveTitle = strings.TrimSpace(payload.Block.Title)
		}
		effectiveLayoutWidth := nullableString(layoutWidth)
		if effectiveLayoutWidth == nil && strings.TrimSpace(payload.Block.LayoutWidth) != "" {
			effectiveLayoutWidth = layoutGridStringPtr(payload.Block.LayoutWidth)
		}

		block := LayoutGridBlock{
			ID:            blockID,
			DatacenterID:  datacenterID,
			IsletID:       nullableInt(isletID),
			IsletName:     effectiveIsletName,
			Title:         effectiveTitle,
			LayoutWidth:   effectiveLayoutWidth,
			DisplayOrder:  displayOrder,
			SchemaVersion: effectiveSchemaVersion,
			Grid:          [][]LayoutGridCell{},
		}
		builds = append(builds, layoutGridBlockBuild{block: block, sourceGrid: payload.Grid})
		blockIDs = append(blockIDs, blockID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	bindings, err := h.layoutGridPlenumBindings(r, blockIDs)
	if err != nil {
		return nil, nil, err
	}
	positionsByKey := layoutGridPositionsByKey(positions)
	blocks := make([]LayoutGridBlock, 0, len(builds))
	for _, build := range builds {
		block := build.block
		block.Grid = make([][]LayoutGridCell, len(build.sourceGrid))
		if block.IsletID == nil && layoutGridHasPositionCells(build.sourceGrid) {
			warnings = append(warnings, fmt.Sprintf("%s / %s: isola non collegata ai dati Grappa", dc.Name, block.Title))
		}
		for rowIndex, row := range build.sourceGrid {
			block.Grid[rowIndex] = make([]LayoutGridCell, len(row))
			for colIndex, sourceCell := range row {
				cell := layoutGridAPICell(sourceCell)
				switch sourceCell.Type {
				case "position":
					if sourceCell.Pos != nil && block.IsletID != nil {
						position, ok := positionsByKey[layoutGridPositionKey{isletID: *block.IsletID, num: *sourceCell.Pos}]
						if ok {
							cell.PositionID = layoutGridIntPtr(position.ID)
							cell.PositionStatus = layoutGridStringPtr(position.Status)
							cell.PositionType = layoutGridStringPtr(position.Type)
							cell.RackID = position.RackID
							cell.RackName = position.RackName
							cell.RackType = position.RackType
							cell.RackPos = position.RackPos
						} else {
							warnings = append(warnings, fmt.Sprintf("%s / %s: posizione %d non trovata nei dati Grappa", dc.Name, block.Title, *sourceCell.Pos))
						}
					}
				case "plenum":
					binding, ok := bindings[layoutGridCellKey{blockID: block.ID, rowIndex: rowIndex, colIndex: colIndex}]
					if ok {
						cell.PlenumID = layoutGridIntPtr(binding.plenumID)
						cell.PlenumName = binding.plenumName
						cell.PlenumStatus = binding.plenumStatus
						if cell.PlenumType == nil {
							cell.PlenumType = binding.plenumType
						}
					} else {
						plenumType := ""
						if cell.PlenumType != nil {
							plenumType = *cell.PlenumType
						}
						warnings = append(warnings, fmt.Sprintf("%s / %s: plenum %s non collegato", dc.Name, block.Title, strings.TrimSpace(plenumType)))
					}
				}
				block.Grid[rowIndex][colIndex] = cell
			}
		}
		blocks = append(blocks, block)
	}
	return blocks, warnings, nil
}

func (h *Handler) layoutGridPlenumBindings(r *http.Request, blockIDs []int) (map[layoutGridCellKey]layoutGridPlenumLiveBinding, error) {
	result := map[layoutGridCellKey]layoutGridPlenumLiveBinding{}
	if len(blockIDs) == 0 {
		return result, nil
	}
	query := fmt.Sprintf(`
		SELECT bp.layout_block_id, bp.row_index, bp.col_index, bp.plenum_id,
		       p.name, p.status, bp.plenum_type
		FROM dcim_layout_block_plenums bp
		JOIN plenums p ON p.id = bp.plenum_id AND p.datacenter_id = bp.datacenter_id
		WHERE bp.layout_block_id IN (%s)`, placeholders(len(blockIDs)))
	args := make([]any, len(blockIDs))
	for i, id := range blockIDs {
		args[i] = id
	}
	rows, err := h.grappa.QueryContext(r.Context(), query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var blockID, rowIndex, colIndex, plenumID int
		var name, status, plenumType sql.NullString
		if err := rows.Scan(&blockID, &rowIndex, &colIndex, &plenumID, &name, &status, &plenumType); err != nil {
			return result, err
		}
		result[layoutGridCellKey{blockID: blockID, rowIndex: rowIndex, colIndex: colIndex}] = layoutGridPlenumLiveBinding{
			plenumID:     plenumID,
			plenumName:   nullableString(name),
			plenumStatus: nullableString(status),
			plenumType:   nullableString(plenumType),
		}
	}
	return result, rows.Err()
}

func layoutGridPositionsByKey(positions []Position) map[layoutGridPositionKey]Position {
	result := map[layoutGridPositionKey]Position{}
	for _, position := range positions {
		key := layoutGridPositionKey{isletID: position.IsletID, num: position.Num}
		if _, exists := result[key]; !exists {
			result[key] = position
		}
	}
	return result
}

func layoutGridAPICell(source layoutGridCell) LayoutGridCell {
	cell := LayoutGridCell{Type: strings.TrimSpace(source.Type)}
	if source.Pos != nil {
		cell.Pos = layoutGridIntPtr(*source.Pos)
	}
	if strings.TrimSpace(source.Text) != "" {
		cell.Text = layoutGridStringPtr(source.Text)
	}
	if strings.TrimSpace(source.PlenumType) != "" {
		cell.PlenumType = layoutGridStringPtr(source.PlenumType)
	}
	return cell
}

func layoutGridHasPositionCells(grid [][]layoutGridCell) bool {
	for _, row := range grid {
		for _, cell := range row {
			if cell.Type == "position" {
				return true
			}
		}
	}
	return false
}

func layoutGridIntPtr(value int) *int {
	v := value
	return &v
}

func layoutGridStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
