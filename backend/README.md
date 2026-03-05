# Backend (Phase 0/1 Scaffold)

This directory contains the first migration milestone backend scaffold:

- Gin HTTP server
- PostgreSQL connection pool
- SQL migration runner (`backend/migrations`)
- JWT access/refresh token service
- Auth APIs (`/auth/*`) with local `dev/login`
- Player snapshot API (`GET /player/snapshot`)

## Quick start

```bash
cd backend
cp .env.example .env
# edit JWT_SECRET / DATABASE_URL if needed
export $(grep -v '^#' .env | xargs)
go run ./cmd/api
```

## Endpoints

- `GET /healthz`
- `GET /auth/linux-do/authorize`
- `GET /auth/linux-do/callback`
- `POST /auth/dev/login` (local testing only, can be disabled)
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /auth/me` (requires Bearer access token)
- `GET /player/snapshot` (requires Bearer access token)
- `GET /rankings?type=realm&scope=global&limit=50` (requires Bearer access token)
- `GET /rankings/friends?type=realm&limit=50` (requires Bearer access token)
- `GET /rankings/self?type=realm&scope=friends` (requires Bearer access token)
- `GET /rankings/follows?limit=100` (requires Bearer access token)
- `POST /rankings/follows` (requires Bearer access token)
- `DELETE /rankings/follows?targetUserId=` (requires Bearer access token)
- `GET /auction/list?limit=20&offset=0` (requires Bearer access token)
- `POST /auction/create` (requires Bearer access token)
- `POST /auction/cancel` (requires Bearer access token)
- `POST /auction/buy` (requires Bearer access token)
- `POST /auction/bid` (requires Bearer access token)
- `POST /auction/accept-bid` (requires Bearer access token)
- `GET /auction/my-orders?limit=20` (requires Bearer access token)
- `WS /chat/connect?accessToken=...`
- `GET /chat/history?channel=world&limit=50` (requires Bearer access token)
- `GET /chat/mute-status` (requires Bearer access token)
- `POST /chat/report` (requires Bearer access token)
- `GET /chat/admin/mutes?targetLinuxDoUserId=&limit=50` (requires Bearer access token + chat admin)
- `POST /chat/admin/mute` (requires Bearer access token + chat admin)
- `POST /chat/admin/unmute` (requires Bearer access token + chat admin)
- `GET /chat/admin/block-words?includeDisabled=true&limit=200` (requires Bearer access token + chat admin)
- `POST /chat/admin/block-words` (requires Bearer access token + chat admin)
- `DELETE /chat/admin/block-words?word=` (requires Bearer access token + chat admin)
- `POST /game/cultivation/once` (requires Bearer access token)
- `POST /game/cultivation/until-breakthrough` (requires Bearer access token)
- `POST /game/breakthrough` (requires Bearer access token)
- `POST /game/exploration/start` (requires Bearer access token)
- `POST /game/alchemy/craft` (requires Bearer access token)
- `POST /game/gacha/draw` (requires Bearer access token)
- `POST /game/inventory/equipment/sell` (requires Bearer access token)
- `POST /game/inventory/equipment/sell-batch` (requires Bearer access token)
- `POST /game/inventory/pet/release` (requires Bearer access token)
- `POST /game/inventory/pet/release-batch` (requires Bearer access token)
- `POST /game/inventory/pet/upgrade` (requires Bearer access token)
- `POST /game/inventory/pet/evolve` (requires Bearer access token)
- `POST /game/item/use` (requires Bearer access token)
- `POST /game/dungeon/start` (requires Bearer access token)
- `POST /game/dungeon/next-turn` (requires Bearer access token)
- `POST /game/inventory/equipment/equip` (requires Bearer access token)
- `POST /game/inventory/equipment/unequip` (requires Bearer access token)
- `POST /game/inventory/equipment/enhance` (requires Bearer access token)
- `POST /game/inventory/equipment/reforge` (requires Bearer access token)

## Notes

- Linux.do OAuth now supports authorize + callback + local JWT issuance.
- OAuth userinfo field extraction uses a tolerant mapping (`sub/id/user_id/uid` etc.) to reduce provider response coupling.
- Dungeon next-turn supports optional payload:
  - `{"selectedOptionId":"..."}`
  - `{"refreshOptions":true}`
- Chat admin users are configured by `CHAT_ADMIN_USER_IDS` (comma-separated Linux.do user IDs).
- Chat blocked words are stored in `chat_block_words` and can be managed via chat admin APIs.
- Chat admin mute/unmute/block-word actions are audited in `risk_events` (`event_type=chat_admin_action`).
- Expired auction orders are swept automatically by worker:
  - `AUCTION_SWEEP_INTERVAL_SECONDS`
  - `AUCTION_SWEEP_BATCH_SIZE`
- Auction seller can accept current highest bid to settle the order via `POST /auction/accept-bid`.
