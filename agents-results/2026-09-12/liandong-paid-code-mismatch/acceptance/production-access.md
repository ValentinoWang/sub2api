# Production access and recharge inventory readback

- The user authorized searching `/Users/vsiyo/Desktop/Key` for the production SSH key.
- `MacbookAir.pem` successfully authenticated as `ubuntu` to `43.156.50.78`, with existing host-key checking enforced. File mode is 0600. No key bytes were displayed or copied.
- `ylf_HK_MacbookAir.pem` did not authenticate to this account. The earlier `sub2api-production` alias pointed to an older host and is not the current website deployment authority; no SSH configuration was changed.
- Remote hostname: `VM-0-7-ubuntu`. Application: container `sub2api`, image `sub2api-local:0.2.5-8c9390cb6e77`, bound to localhost port 8080. Compose working directory: `/home/ubuntu/sub2api/deploy`.
- Read-only PostgreSQL query: zero Liandong product mappings, zero Liandong restock batches. Online payment setting true; shop purchase setting false and shop URL empty.
- No persisted merchant configuration, restock state, or administrator API key was found under their known setting keys.
- Application environment: merchant token and code secret empty, product list `[]`, merchant API base `https://ldxp.cn`.
- The production toolkit page redirects to administrator login in the available browser session. No credentials were generated, no password was changed, and no production container or payment configuration was modified.

Server SSH access is resolved. Merchant authentication and product/inventory setup remain necessary before enabling continuous restock or reopening the purchase link. Dev/prod should share code and grant rules while retaining separate inventories and ledgers, as recommended in the conversation.
