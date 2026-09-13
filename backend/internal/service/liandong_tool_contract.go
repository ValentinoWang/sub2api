package service

// LiandongToolkitConnectivityResult reports a read-only merchant probe.
// It deliberately has no token, URL, or response-body fields.
type LiandongToolkitConnectivityResult struct {
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	ReadOnly   bool   `json:"read_only"`
	Message    string `json:"message,omitempty"`
}

// LiandongToolkitGood is the safe subset of a remote LDXP card good exposed
// to the administrator UI.
type LiandongToolkitGood struct {
	GoodsID      int64  `json:"goods_id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	CurrentStock int    `json:"current_stock"`
}

type LiandongToolkitGoodsResult struct {
	Goods []LiandongToolkitGood `json:"goods"`
}
