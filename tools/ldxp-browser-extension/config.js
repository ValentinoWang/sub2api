// This personal extension has two fixed sites. Product IDs and amounts are read
// from each site's authorized device configuration, never copied between sites.
export const DEFAULT_STOCK_TARGET = 999;
export const PROJECT_DIRECTORY = '/Users/vsiyo/Desktop/Opensource_Tool/Sub2api/tools/ldxp-browser-extension';
export const SITE_CONFIG = Object.freeze([
  Object.freeze({ site: 'http://127.0.0.1:8080', label: '本地站', kind: '测试商品', id: 'local', adminURL: 'http://127.0.0.1:8080/admin/tools/ldxp' }),
  Object.freeze({ site: 'https://ai.rest2build.lol', label: '生产站', kind: '正式商品', id: 'production', adminURL: 'https://ai.rest2build.lol/admin/tools/ldxp' }),
]);
