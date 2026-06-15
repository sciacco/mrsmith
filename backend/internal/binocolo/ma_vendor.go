package binocolo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type maShareholder struct {
	TaxCode      string
	Name         string
	Surname      string
	CompanyName  string
	PercentShare float64
}

func parseMATargetsFromVendorData(raw json.RawMessage) ([]MATarget, error) {
	items, err := vendorDataItems(raw)
	if err != nil {
		return nil, err
	}
	targets := make([]MATarget, 0, len(items))
	for _, item := range items {
		target, err := normalizeMATarget(item)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func vendorDataItems(raw json.RawMessage) ([]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("decode vendor company data: %w", err)
	}
	for _, key := range []string{"data", "results", "items"} {
		nested, ok := object[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(nested, &list); err == nil {
			return list, nil
		}
	}
	return []json.RawMessage{raw}, nil
}

func normalizeMATarget(raw json.RawMessage) (MATarget, error) {
	object, err := decodeVendorObject(raw)
	if err != nil {
		return MATarget{}, err
	}

	turnover := vendorInt(object, "balanceSheets.last.turnover", "ecofin.turnover", "turnover")
	turnoverYear := vendorInt(object, "balanceSheets.last.year", "ecofin.turnoverYear")
	employees := vendorInt(object, "balanceSheets.last.employees", "employees.employeeRange.code", "employees")
	atecoCode := firstVendorString(object,
		"atecoClassification.ateco.code",
		"atecoClassification.ateco2022.code",
		"ateco.code",
	)
	atecoDescription := firstVendorString(object,
		"atecoClassification.ateco.description",
		"atecoClassification.ateco2022.description",
		"ateco.description",
	)

	return MATarget{
		VendorID:         firstVendorString(object, "id", "companyDetails.openapiNumber"),
		CompanyName:      firstVendorString(object, "companyName", "companyDetails.companyName"),
		VATCode:          firstVendorString(object, "vatCode", "companyDetails.vatCode"),
		TaxCode:          firstVendorString(object, "taxCode", "companyDetails.taxCode"),
		Province:         firstVendorString(object, "address.registeredOffice.province", "address.province.code", "address.province"),
		Town:             firstVendorString(object, "address.registeredOffice.town", "address.town"),
		ActivityStatus:   firstVendorString(object, "activityStatus", "companyStatus.activityStatus", "companyStatus.status"),
		Turnover:         turnover,
		TurnoverYear:     turnoverYear,
		Employees:        employees,
		AtecoCode:        normalizeAtecoCode(atecoCode),
		AtecoDescription: atecoDescription,
		VendorPayload:    append(json.RawMessage(nil), raw...),
	}, nil
}

func decodeVendorObject(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("decode vendor company row: %w", err)
	}
	return object, nil
}

func firstVendorString(object map[string]any, paths ...string) string {
	for _, path := range paths {
		value, ok := vendorPath(object, path)
		if !ok {
			continue
		}
		if text := vendorString(value); text != "" {
			return text
		}
	}
	return ""
}

func vendorInt(object map[string]any, paths ...string) *int {
	for _, path := range paths {
		value, ok := vendorPath(object, path)
		if !ok {
			continue
		}
		if parsed, ok := vendorNumber(value); ok {
			intValue := int(math.Round(parsed))
			return &intValue
		}
	}
	return nil
}

func vendorPath(object map[string]any, path string) (any, bool) {
	var current any = object
	for _, part := range strings.Split(path, ".") {
		item, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := item[part]
		if !ok {
			return nil, false
		}
		current = value
	}
	return current, true
}

func vendorString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == math.Trunc(typed) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func vendorNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case string:
		value := strings.TrimSpace(strings.ReplaceAll(typed, " ", ""))
		if value == "" {
			return 0, false
		}
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
		parsed, err := strconv.ParseFloat(value, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func targetShareholders(target MATarget) []maShareholder {
	if len(target.VendorPayload) == 0 {
		return nil
	}
	object, err := decodeVendorObject(target.VendorPayload)
	if err != nil {
		return nil
	}
	return extractShareholders(object)
}

func extractShareholders(object map[string]any) []maShareholder {
	if raw, ok := object["shareHolders"]; ok {
		if list, ok := raw.([]any); ok {
			return extractFlatShareholders(list, 0)
		}
	}
	if raw, ok := object["shareholders"]; ok {
		if list, ok := raw.([]any); ok {
			return extractNestedShareholders(list)
		}
	}
	return nil
}

func extractFlatShareholders(list []any, inheritedPercent float64) []maShareholder {
	out := make([]maShareholder, 0, len(list))
	for _, item := range list {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		percent := inheritedPercent
		if parsed, ok := vendorNumber(object["percentShare"]); ok {
			percent = parsed
		}
		out = append(out, maShareholder{
			TaxCode:      vendorString(object["taxCode"]),
			Name:         vendorString(object["name"]),
			Surname:      vendorString(object["surname"]),
			CompanyName:  vendorString(object["companyName"]),
			PercentShare: percent,
		})
	}
	return out
}

func extractNestedShareholders(list []any) []maShareholder {
	out := make([]maShareholder, 0, len(list))
	for _, item := range list {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		percent, _ := vendorNumber(object["percentShare"])
		info, ok := object["shareholdersInformation"].([]any)
		if !ok || len(info) == 0 {
			out = append(out, maShareholder{PercentShare: percent})
			continue
		}
		out = append(out, extractFlatShareholders(info, percent)...)
	}
	return out
}

func shareholderPercentsLabel(shareholders []maShareholder) string {
	parts := make([]string, 0, len(shareholders))
	for _, shareholder := range shareholders {
		if shareholder.PercentShare <= 0 {
			continue
		}
		parts = append(parts, strconv.FormatFloat(shareholder.PercentShare, 'f', -1, 64)+"%")
	}
	if len(parts) == 0 {
		return "quote non disponibili"
	}
	return strings.Join(parts, ", ")
}

func agesLabel(ages []int, partial bool) string {
	parts := make([]string, 0, len(ages)+1)
	for _, age := range ages {
		parts = append(parts, strconv.Itoa(age))
	}
	if partial {
		parts = append(parts, "parziale")
	}
	return strings.Join(parts, ", ")
}
