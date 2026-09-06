package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	goodsListPath       = "/merchantApi/Goods/list"
	unsoldInventoryPath = "/merchantApi/goodsCardStorage/list"
)

// RemoteCardGood is the stable subset needed by the CLI while retaining the
// merchant's name, type, and current stock fields.
type RemoteCardGood struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	CurrentStock int    `json:"current_stock"`
}

// RestockPlanItem is a server-safe plan. It contains no generated redemption
// code; the Sub2API job service remains the only code issuer.
type RestockPlanItem struct {
	GoodsID     int64   `json:"goods_id"`
	CNYAmount   int     `json:"cny_amount"`
	USDCredit   float64 `json:"usd_credit"`
	Enabled     bool    `json:"enabled"`
	TargetStock int     `json:"target_stock"`
	UnsoldStock int     `json:"unsold_stock"`
	Needed      int     `json:"needed"`
}

type RestockPreview struct {
	TargetStock                 int               `json:"target_stock"`
	UploadSegment               int               `json:"upload_segment"`
	Items                       []RestockPlanItem `json:"items"`
	UnconfiguredRemoteCardGoods []RemoteCardGood  `json:"unconfigured_remote_card_goods"`
}

type goodsPage struct {
	Goods    []RemoteCardGood
	RowCount int
	Total    int
	HasTotal bool
	PageSize int
}

type ldxpClient struct {
	api *apiClient
}

func newLDXPClient(cfg *Config) (*ldxpClient, error) {
	api, err := newAPIClient("LDXP", cfg.LDXP.BaseURL, cfg.LDXP.MerchantToken, "Merchant-Token", ldxpResponseLimit, false)
	if err != nil {
		return nil, err
	}
	return &ldxpClient{api: api}, nil
}

func (c *ldxpClient) listGoods(ctx context.Context) ([]RemoteCardGood, error) {
	const requestedPageSize = 1000
	const maxPages = 10000
	seen := make(map[int64]struct{})
	goods := make([]RemoteCardGood, 0)
	rawRows := 0
	for page := 1; page <= maxPages; page++ {
		envelope, _, err := c.api.callJSON(ctx, "POST", goodsListPath, map[string]any{
			"current":  page,
			"pageSize": requestedPageSize,
			"is_proxy": 0,
		})
		if err != nil {
			return nil, err
		}
		parsed, err := parseGoodsPage(envelope.Data)
		if err != nil {
			return nil, fmt.Errorf("decode LDXP goods page %d: %w", page, err)
		}
		rawRows += parsed.RowCount
		for _, good := range parsed.Goods {
			if _, exists := seen[good.ID]; exists {
				continue
			}
			seen[good.ID] = struct{}{}
			goods = append(goods, good)
		}
		if parsed.RowCount == 0 {
			if parsed.HasTotal && rawRows < parsed.Total {
				return nil, fmt.Errorf("LDXP goods pagination ended before total %d rows", parsed.Total)
			}
			return goods, nil
		}
		if parsed.HasTotal && rawRows >= parsed.Total {
			return goods, nil
		}
		pageSize := parsed.PageSize
		if pageSize <= 0 {
			pageSize = requestedPageSize
		}
		if !parsed.HasTotal && parsed.RowCount < pageSize {
			return goods, nil
		}
	}
	return nil, errors.New("LDXP goods pagination exceeded the safety limit")
}

func (c *ldxpClient) unsoldStock(ctx context.Context, goodsID int64) (int, error) {
	envelope, _, err := c.api.callJSON(ctx, "POST", unsoldInventoryPath, map[string]any{
		"goods_id": goodsID,
		"current":  1,
		"pageSize": 1,
		"status":   "0",
		"first":    "",
		"keywords": "",
	})
	if err != nil {
		return 0, err
	}
	stock, err := parseInventoryTotal(envelope.Data)
	if err != nil {
		return 0, fmt.Errorf("decode LDXP unsold inventory for goods %d: %w", goodsID, err)
	}
	return stock, nil
}

func buildRestockPreview(ctx context.Context, cfg *Config, client *ldxpClient) (*RestockPreview, error) {
	goods, err := client.listGoods(ctx)
	if err != nil {
		return nil, err
	}
	configured := make(map[int64]struct{}, len(cfg.ProductMappings))
	for _, mapping := range cfg.ProductMappings {
		configured[mapping.GoodsID] = struct{}{}
	}
	unconfigured := make([]RemoteCardGood, 0)
	for _, good := range goods {
		if _, exists := configured[good.ID]; !exists {
			unconfigured = append(unconfigured, good)
		}
	}

	items := make([]RestockPlanItem, 0)
	for _, mapping := range cfg.ProductMappings {
		if !mapping.Enabled {
			continue
		}
		stock, err := client.unsoldStock(ctx, mapping.GoodsID)
		if err != nil {
			return nil, err
		}
		needed := cfg.TargetStock - stock
		if needed < 0 {
			needed = 0
		}
		items = append(items, RestockPlanItem{
			GoodsID:     mapping.GoodsID,
			CNYAmount:   mapping.CNYAmount,
			USDCredit:   mapping.USDCredit,
			Enabled:     true,
			TargetStock: cfg.TargetStock,
			UnsoldStock: stock,
			Needed:      needed,
		})
	}
	return &RestockPreview{
		TargetStock:                 cfg.TargetStock,
		UploadSegment:               cfg.UploadSegment,
		Items:                       items,
		UnconfiguredRemoteCardGoods: unconfigured,
	}, nil
}

func parseGoodsPage(raw json.RawMessage) (*goodsPage, error) {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return nil, err
	}
	page, found, err := parseGoodsContainer(value)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("expected a goods list array")
	}
	return page, nil
}

func parseGoodsContainer(value any) (*goodsPage, bool, error) {
	switch typed := value.(type) {
	case []any:
		goods, err := parseGoodsRows(typed)
		if err != nil {
			return nil, true, err
		}
		return &goodsPage{Goods: goods, RowCount: len(typed)}, true, nil
	case map[string]any:
		page := &goodsPage{}
		if total, ok, err := numberField(typed, "total", "total_count", "totalCount", "count"); err != nil {
			return nil, true, err
		} else if ok {
			if total < 0 {
				return nil, true, errors.New("goods total must be non-negative")
			}
			page.Total, page.HasTotal = total, true
		}
		if size, ok, err := numberField(typed, "pageSize", "page_size", "size"); err != nil {
			return nil, true, err
		} else if ok {
			if size < 0 {
				return nil, true, errors.New("goods page size must be non-negative")
			}
			page.PageSize = size
		}
		for _, key := range []string{"records", "rows", "list", "items", "goods", "data", "result"} {
			nested, exists := typed[key]
			if !exists {
				continue
			}
			child, found, err := parseGoodsContainer(nested)
			if err != nil {
				return nil, true, err
			}
			if !found {
				return nil, true, fmt.Errorf("goods field %q is not a list", key)
			}
			if !page.HasTotal && child.HasTotal {
				page.Total, page.HasTotal = child.Total, true
			}
			if page.PageSize == 0 {
				page.PageSize = child.PageSize
			}
			page.Goods = child.Goods
			page.RowCount = child.RowCount
			return page, true, nil
		}
	}
	return nil, false, nil
}

func parseGoodsRows(rows []any) ([]RemoteCardGood, error) {
	goods := make([]RemoteCardGood, 0, len(rows))
	for index, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("goods row %d is not an object", index)
		}
		good, isCard, err := parseRemoteGood(row)
		if err != nil {
			return nil, fmt.Errorf("goods row %d: %w", index, err)
		}
		if isCard {
			goods = append(goods, *good)
		}
	}
	return goods, nil
}

func parseRemoteGood(row map[string]any) (*RemoteCardGood, bool, error) {
	typeValue, ok := fieldValue(row, "type", "goods_type", "goodsType", "product_type", "productType")
	if !ok {
		if card, exists := fieldValue(row, "is_card", "isCard"); exists {
			if boolValue, ok := card.(bool); ok {
				if boolValue {
					typeValue = "card"
				} else {
					typeValue = "non-card"
				}
			} else {
				return nil, false, errors.New("type is not a string or number")
			}
		} else {
			return nil, false, errors.New("goods is missing type")
		}
	}
	typeName, err := stringifyValue(typeValue)
	if err != nil {
		return nil, false, fmt.Errorf("invalid type: %w", err)
	}
	if !isCardType(typeName) {
		return nil, false, nil
	}
	idValue, ok := fieldValue(row, "id", "goods_id", "goodsId")
	if !ok {
		return nil, true, errors.New("card goods is missing numeric id")
	}
	id, err := integer64Value(idValue)
	if err != nil || id <= 0 {
		return nil, true, errors.New("card goods id must be a positive integer")
	}
	nameValue, ok := fieldValue(row, "name", "goods_name", "goodsName", "title")
	if !ok {
		return nil, true, errors.New("card goods is missing name")
	}
	name, err := stringifyValue(nameValue)
	if err != nil {
		return nil, true, fmt.Errorf("invalid name: %w", err)
	}
	stockValue, ok := fieldValue(row, "current_stock", "currentStock", "stock", "goods_stock", "goodsStock", "stock_num", "stockNum", "inventory", "inventory_count")
	if !ok {
		return nil, true, errors.New("card goods is missing current stock")
	}
	stock, err := integerValue(stockValue)
	if err != nil || stock < 0 {
		return nil, true, errors.New("card goods current stock must be a non-negative integer")
	}
	return &RemoteCardGood{ID: id, Name: name, Type: typeName, CurrentStock: stock}, true, nil
}

func isCardType(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "1" || normalized == "card" || normalized == "goods_card" || strings.Contains(normalized, "card") || strings.Contains(normalized, "卡")
}

func parseInventoryTotal(raw json.RawMessage) (int, error) {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return 0, err
	}
	stock, found, err := findInventoryTotal(value)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errors.New("expected inventory data with total")
	}
	if stock < 0 {
		return 0, errors.New("inventory total must be non-negative")
	}
	return stock, nil
}

func findInventoryTotal(value any) (int, bool, error) {
	switch typed := value.(type) {
	case []any:
		return len(typed), true, nil
	case map[string]any:
		for _, key := range []string{"total", "total_count", "totalCount", "count"} {
			if field, ok := typed[key]; ok {
				number, err := integerValue(field)
				if err != nil {
					return 0, true, fmt.Errorf("inventory %s: %w", key, err)
				}
				return number, true, nil
			}
		}
		for _, key := range []string{"data", "result", "records", "rows", "list", "items"} {
			if nested, ok := typed[key]; ok {
				return findInventoryTotal(nested)
			}
		}
	}
	return 0, false, nil
}

func decodeJSONValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func fieldValue(row map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func numberField(row map[string]any, keys ...string) (int, bool, error) {
	value, ok := fieldValue(row, keys...)
	if !ok {
		return 0, false, nil
	}
	number, err := integerValue(value)
	return number, true, err
}

func integerValue(value any) (int, error) {
	parsed, err := integer64Value(value)
	if err != nil {
		return 0, err
	}
	if int64(int(parsed)) != parsed {
		return 0, errors.New("integer is outside platform range")
	}
	return int(parsed), nil
}

func integer64Value(value any) (int64, error) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, errors.New("number is not an integer")
		}
		return int64(typed), nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", value)
	}
}

func stringifyValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case json.Number:
		return string(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("expected string or number, got %T", value)
	}
}
