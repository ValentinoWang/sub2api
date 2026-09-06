package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	liandongToolkitGoodsPath = "/merchantApi/Goods/list"
	liandongToolkitPageSize  = 1000
	liandongToolkitMaxPages  = 10000
)

// TestConfiguration probes the read-only goods endpoint with the stored
// merchant configuration. It reports connectivity as data so a failed probe
// does not expose upstream response text or credentials.
func (s *LiandongRestockService) TestConfiguration(ctx context.Context) (*LiandongToolkitConnectivityResult, error) {
	result := &LiandongToolkitConnectivityResult{ReadOnly: true}
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_SERVICE_UNAVAILABLE", "LDXP toolkit service is unavailable")
	}
	s.configMu.RLock()
	baseURL := strings.TrimSpace(s.baseURL)
	token := strings.TrimSpace(s.token)
	clientAvailable := s.httpClient != nil
	s.configMu.RUnlock()
	result.Configured = baseURL != "" && token != ""
	if !result.Configured {
		result.Message = "LDXP merchant configuration is incomplete"
		return result, nil
	}
	if !clientAvailable {
		result.Message = "LDXP merchant connectivity client is unavailable"
		return result, nil
	}
	if _, err := s.post(ctx, liandongToolkitGoodsPath, map[string]any{
		"current":  1,
		"pageSize": 1,
		"is_proxy": 0,
	}); err != nil {
		result.Message = "LDXP merchant connectivity check failed"
		return result, nil
	}
	result.Reachable = true
	result.Message = "LDXP merchant connectivity check succeeded"
	return result, nil
}

// ListGoods returns remote card goods and always sends is_proxy=0. The fixed
// request shape is kept inside the domain service so callers cannot broaden
// the query to proxy goods.
func (s *LiandongRestockService) ListGoods(ctx context.Context) (*LiandongToolkitGoodsResult, error) {
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_SERVICE_UNAVAILABLE", "LDXP toolkit service is unavailable")
	}
	s.configMu.RLock()
	configured := strings.TrimSpace(s.baseURL) != "" && strings.TrimSpace(s.token) != ""
	clientAvailable := s.httpClient != nil
	s.configMu.RUnlock()
	if !configured || !clientAvailable {
		return nil, infraerrors.ServiceUnavailable("LDXP_NOT_CONFIGURED", "LDXP merchant configuration is unavailable")
	}

	seen := make(map[int64]struct{})
	goods := make([]LiandongToolkitGood, 0)
	rawRows := 0
	for page := 1; page <= liandongToolkitMaxPages; page++ {
		result, err := s.post(ctx, liandongToolkitGoodsPath, map[string]any{
			"current":  page,
			"pageSize": liandongToolkitPageSize,
			"is_proxy": 0,
		})
		if err != nil {
			return nil, err
		}
		parsed, err := parseLiandongToolkitGoodsPage(result.Data)
		if err != nil {
			return nil, err
		}
		rawRows += parsed.rowCount
		for _, good := range parsed.rows {
			if _, exists := seen[good.GoodsID]; exists {
				continue
			}
			seen[good.GoodsID] = struct{}{}
			goods = append(goods, good)
		}
		if parsed.rowCount == 0 {
			if parsed.hasTotal && rawRows < parsed.total {
				return nil, errors.New("LDXP goods pagination ended before the reported total")
			}
			break
		}
		if parsed.hasTotal && rawRows >= parsed.total {
			break
		}
		pageSize := parsed.pageSize
		if pageSize <= 0 {
			pageSize = liandongToolkitPageSize
		}
		if !parsed.hasTotal && parsed.rowCount < pageSize {
			break
		}
		if page == liandongToolkitMaxPages {
			return nil, errors.New("LDXP goods pagination exceeded the safety limit")
		}
	}
	return &LiandongToolkitGoodsResult{Goods: goods}, nil
}

type liandongToolkitGoodsPage struct {
	rows     []LiandongToolkitGood
	rowCount int
	total    int
	hasTotal bool
	pageSize int
}

func parseLiandongToolkitGoodsPage(raw json.RawMessage) (*liandongToolkitGoodsPage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, errors.New("LDXP goods response has no data")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("LDXP goods response data is invalid")
	}
	rows, total, hasTotal, pageSize, found, err := findLiandongToolkitGoodsRows(value)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("LDXP goods response does not contain a list")
	}
	result := &liandongToolkitGoodsPage{
		rowCount: len(rows),
		total:    total,
		hasTotal: hasTotal,
		pageSize: pageSize,
	}
	for _, rawRow := range rows {
		good, isCard, err := parseLiandongToolkitGood(rawRow)
		if err != nil {
			return nil, err
		}
		if isCard {
			result.rows = append(result.rows, good)
		}
	}
	return result, nil
}

func findLiandongToolkitGoodsRows(value any) ([]any, int, bool, int, bool, error) {
	switch typed := value.(type) {
	case []any:
		return typed, 0, false, 0, true, nil
	case map[string]any:
		totalValue, hasTotal, err := liandongToolkitNumberField(typed, "total", "total_count", "totalCount", "count")
		if err != nil {
			return nil, 0, false, 0, false, err
		}
		if hasTotal && (totalValue < 0 || totalValue > int64(^uint(0)>>1)) {
			return nil, 0, false, 0, false, errors.New("LDXP goods total is invalid")
		}
		total := int(totalValue)
		pageSizeValue, _, err := liandongToolkitNumberField(typed, "pageSize", "page_size", "size")
		if err != nil {
			return nil, 0, false, 0, false, err
		}
		if pageSizeValue < 0 || pageSizeValue > int64(^uint(0)>>1) {
			return nil, 0, false, 0, false, errors.New("LDXP goods page size is invalid")
		}
		pageSize := int(pageSizeValue)
		for _, key := range []string{"records", "rows", "list", "items", "goods"} {
			if nested, exists := typed[key]; exists {
				rows, nestedTotal, nestedHasTotal, nestedPageSize, found, err := findLiandongToolkitGoodsRows(nested)
				if err != nil {
					return nil, 0, false, 0, false, err
				}
				if found {
					if !hasTotal && nestedHasTotal {
						total, hasTotal = nestedTotal, true
					}
					if pageSize == 0 {
						pageSize = nestedPageSize
					}
					return rows, total, hasTotal, pageSize, true, nil
				}
			}
		}
		for _, key := range []string{"data", "result", "payload"} {
			if nested, exists := typed[key]; exists {
				rows, nestedTotal, nestedHasTotal, nestedPageSize, found, err := findLiandongToolkitGoodsRows(nested)
				if err != nil {
					return nil, 0, false, 0, false, err
				}
				if found {
					if !hasTotal && nestedHasTotal {
						total, hasTotal = nestedTotal, true
					}
					if pageSize == 0 {
						pageSize = nestedPageSize
					}
					return rows, total, hasTotal, pageSize, true, nil
				}
			}
		}
	}
	return nil, 0, false, 0, false, nil
}

func parseLiandongToolkitGood(value any) (LiandongToolkitGood, bool, error) {
	row, ok := value.(map[string]any)
	if !ok {
		return LiandongToolkitGood{}, false, errors.New("LDXP goods list contains a non-object row")
	}
	goodsID, found, err := liandongToolkitNumberField(row, "goods_id", "goodsId", "id")
	if err != nil {
		return LiandongToolkitGood{}, false, err
	}
	if !found || goodsID <= 0 {
		return LiandongToolkitGood{}, false, errors.New("LDXP goods row has no positive numeric ID")
	}
	name := liandongToolkitStringField(row, "name", "goods_name", "goodsName", "title")
	typeName := liandongToolkitStringField(row, "type", "goods_type", "goodsType", "type_name")
	if cardFlag, exists := liandongToolkitBooleanField(row, "is_card", "isCard"); exists && !cardFlag {
		return LiandongToolkitGood{}, false, nil
	}
	if !isLiandongToolkitCardType(typeName) {
		return LiandongToolkitGood{}, false, nil
	}
	stock, foundStock, err := liandongToolkitNumberField(row, "current_stock", "currentStock", "stock", "inventory")
	if err != nil {
		return LiandongToolkitGood{}, false, err
	}
	if !foundStock {
		stock = 0
	}
	return LiandongToolkitGood{
		GoodsID:      int64(goodsID),
		Name:         name,
		Type:         typeName,
		CurrentStock: int(stock),
	}, true, nil
}

func isLiandongToolkitCardType(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}
	if strings.Contains(normalized, "non-card") || strings.Contains(normalized, "subscription") {
		return false
	}
	if strings.Contains(value, "卡") {
		return true
	}
	switch normalized {
	case "card", "card_goods", "goods_card", "virtual_card":
		return true
	case "physical", "service", "membership", "package":
		return false
	default:
		return true
	}
}

func liandongToolkitNumberField(row map[string]any, keys ...string) (int64, bool, error) {
	for _, key := range keys {
		value, exists := row[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case json.Number:
			parsed, err := strconv.ParseInt(string(typed), 10, 64)
			if err != nil {
				return 0, false, fmt.Errorf("LDXP goods numeric field %s is invalid", key)
			}
			return parsed, true, nil
		case float64:
			if typed != float64(int64(typed)) {
				return 0, false, fmt.Errorf("LDXP goods numeric field %s is invalid", key)
			}
			return int64(typed), true, nil
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			if err != nil {
				return 0, false, fmt.Errorf("LDXP goods numeric field %s is invalid", key)
			}
			return parsed, true, nil
		default:
			return 0, false, fmt.Errorf("LDXP goods numeric field %s is invalid", key)
		}
	}
	return 0, false, nil
}

func liandongToolkitStringField(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func liandongToolkitBooleanField(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, exists := row[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case json.Number:
			return typed != "0", true
		case float64:
			return typed != 0, true
		case string:
			return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1", true
		}
	}
	return false, false
}

var _ LiandongToolkitService = (*LiandongRestockService)(nil)
