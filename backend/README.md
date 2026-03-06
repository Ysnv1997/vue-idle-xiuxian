# Backend (Phase 0/1 Scaffold)

This directory contains the first migration milestone backend scaffold:

- Gin HTTP server
- PostgreSQL connection pool
- SQL migration runner (`backend/migrations`)
- JWT access/refresh token service
- Auth APIs (`/api/v1/auth/*`) with local `dev/login`
- Player snapshot API (`GET /api/v1/player/snapshot`)

## Quick start

```bash
cd backend
cp .env.example .env
# edit JWT_SECRET / DATABASE_URL if needed
go run ./cmd/api
```

The backend now auto-loads env files on startup:

- `backend/.env` (when starting from repository root)
- `.env` (when starting from `backend/` directory)

You can force a specific env file with:

```bash
APP_CONFIG_ENV_FILE=/path/to/your.env go run ./cmd/api
```

## Endpoints

All endpoints are served under the `/api/v1` prefix.

- `GET /api/v1/healthz`
- `GET /api/v1/auth/linux-do/authorize`
- `GET /api/v1/auth/linux-do/callback`
- `POST /api/v1/auth/dev/login` (local testing only, can be disabled)
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me` (requires Bearer access token)
- `GET /api/v1/player/snapshot` (requires Bearer access token)
- `GET /api/v1/rankings?type=realm&scope=global&limit=50` (requires Bearer access token)
- `GET /api/v1/rankings/friends?type=realm&limit=50` (requires Bearer access token)
- `GET /api/v1/rankings/self?type=realm&scope=friends` (requires Bearer access token)
- `GET /api/v1/rankings/follows?limit=100` (requires Bearer access token)
- `POST /api/v1/rankings/follows` (requires Bearer access token)
- `DELETE /api/v1/rankings/follows?targetUserId=` (requires Bearer access token)
- `GET /api/v1/auction/list?limit=20&offset=0` (requires Bearer access token)
- `POST /api/v1/auction/create` (requires Bearer access token)
- `POST /api/v1/auction/cancel` (requires Bearer access token)
- `POST /api/v1/auction/buy` (requires Bearer access token)
- `GET /api/v1/auction/my-orders?limit=20` (requires Bearer access token)
- `WS /api/v1/chat/connect?accessToken=...`
- `WS /api/v1/game/realtime/connect?accessToken=...`
- `GET /api/v1/chat/history?channel=world&limit=50` (requires Bearer access token)
- `GET /api/v1/chat/mute-status` (requires Bearer access token)
- `POST /api/v1/chat/report` (requires Bearer access token)
- `GET /api/v1/chat/admin/mutes?targetLinuxDoUserId=&limit=50` (requires Bearer access token + chat admin)
- `POST /api/v1/chat/admin/mute` (requires Bearer access token + chat admin)
- `POST /api/v1/chat/admin/unmute` (requires Bearer access token + chat admin)
- `GET /api/v1/chat/admin/block-words?includeDisabled=true&limit=200` (requires Bearer access token + chat admin)
- `POST /api/v1/chat/admin/block-words` (requires Bearer access token + chat admin)
- `DELETE /api/v1/chat/admin/block-words?word=` (requires Bearer access token + chat admin)
- `GET /api/v1/admin/me` (requires Bearer access token)
- `GET /api/v1/admin/users?limit=200` (requires Bearer access token + `super_admin`)
- `POST /api/v1/admin/users` (requires Bearer access token + `super_admin`)
- `DELETE /api/v1/admin/users?linuxDoUserId=` (requires Bearer access token + `super_admin`)
- `GET /api/v1/admin/runtime-configs?category=&q=&limit=300` (requires Bearer access token + `super_admin|ops_admin`)
- `POST /api/v1/admin/runtime-configs` (requires Bearer access token + `super_admin|ops_admin`)
- `GET /api/v1/admin/runtime-config-audits?key=&category=&limit=200` (requires Bearer access token + `super_admin|ops_admin`)
- `GET /api/v1/recharge/products` (requires Bearer access token)
- `GET /api/v1/recharge/orders?limit=20` (requires Bearer access token)
- `POST /api/v1/recharge/orders` (requires Bearer access token)
- `POST /api/v1/recharge/orders/sync` (requires Bearer access token)
- `GET /api/v1/recharge/callback/credit-linux-do` (Linux.do Credit callback)
- `POST /api/v1/game/cultivation/once` (requires Bearer access token)
- `POST /api/v1/game/cultivation/until-breakthrough` (requires Bearer access token)
- `POST /api/v1/game/breakthrough` (requires Bearer access token)
- `POST /api/v1/game/exploration/start` (requires Bearer access token)
- `POST /api/v1/game/alchemy/craft` (requires Bearer access token)
- `POST /api/v1/game/gacha/draw` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/sell` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/sell-batch` (requires Bearer access token)
- `POST /api/v1/game/inventory/pet/release` (requires Bearer access token)
- `POST /api/v1/game/inventory/pet/release-batch` (requires Bearer access token)
- `POST /api/v1/game/inventory/pet/upgrade` (requires Bearer access token)
- `POST /api/v1/game/inventory/pet/evolve` (requires Bearer access token)
- `POST /api/v1/game/item/use` (requires Bearer access token)
- `POST /api/v1/game/dungeon/start` (requires Bearer access token)
- `POST /api/v1/game/dungeon/next-turn` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/equip` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/unequip` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/enhance` (requires Bearer access token)
- `POST /api/v1/game/inventory/equipment/reforge` (requires Bearer access token)

## Notes

- Linux.do OAuth now supports authorize + callback + local JWT issuance.
- OAuth userinfo field extraction uses a tolerant mapping (`sub/id/user_id/uid` etc.) to reduce provider response coupling.
- Dungeon next-turn supports optional payload:
  - `{"selectedOptionId":"..."}`
  - `{"refreshOptions":true}`
- `CHAT_ADMIN_USER_IDS` is now a bootstrap seed only; real admin list is persisted in `game_admin_users` and managed via admin APIs/UI.
- Admin roles:
  - `super_admin`: full admin permissions
  - `ops_admin`: runtime config management
  - `chat_admin`: chat moderation
- Chat blocked words are stored in `chat_block_words` and can be managed via chat admin APIs.
- Chat admin mute/unmute/block-word actions are audited in `risk_events` (`event_type=chat_admin_action`).
- Runtime config changes are audited in `game_runtime_config_audit_logs`.
- Recharge (Linux.do Credit EasyPay) relies on:
  - `RECHARGE_EPAY_PID`
  - `RECHARGE_EPAY_KEY`
  - `RECHARGE_EPAY_BASE_URL` (default `https://credit.linux.do/epay`)
  - `RECHARGE_NOTIFY_URL` / `RECHARGE_RETURN_URL` (optional, signed but do not override console config)
- Expired auction orders are swept automatically by worker:
  - `AUCTION_SWEEP_INTERVAL_SECONDS`
  - `AUCTION_SWEEP_BATCH_SIZE`
- Hunting runs are advanced by backend worker (in addition to request-triggered sync):
- Game realtime websocket pushes `player.snapshot`, `game.meditation`, `game.hunting`; updates are event-driven by workers and gameplay actions, with a 30s keepalive sync.
- Realtime broker merges burst notifications per user before websocket sync, reducing duplicate push work during rapid combat/resource updates.
- Passive progress middleware now only records activity heartbeat; authoritative progression is driven by workers and game realtime websocket sync.
- `HUNTING_SWEEP_INTERVAL_SECONDS`
- `HUNTING_SWEEP_BATCH_SIZE`
