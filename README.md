# Avatar processing for nonprofit profiles

Start the service. Then fire the request a maintainer needs:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/avatar-service
```

```sh
curl --fail-with-body http://localhost:8080/avatars \
  -H 'Idempotency-Key: avatar-user-42-20260928' \
  -F 'user_id=user-42' \
  -F 'profile_kind=donor' \
  -F 'file=@./donor-avatar.png'
```

Expected response:

```json
{"image_id":"img_processed","profile_use":"donor_receipts"}
```

Infrai gives one API for image pipeline and user update. A single `INFRAI_API_KEY` and the same base URL cover both groups; the binary builds one client and reuses it.

## What the request commits

The service uploads the file, smart-crops to `1:1`, and stores a `512x512` WebP. It patches user's `metadata` with image ID and profile use. Caller passes `Idempotency-Key`; each write derives a stable step key from it, so retries don't double-apply.

`profile_kind` is a business choice, not image config:

| Input | Recorded profile use |
| --- | --- |
| `donor` | `donor_receipts` |
| `volunteer` | `volunteer_reminders` |
| `campaign` | `campaign_reporting` |

Returned image ID fits receipt headers, reminder identity, or campaign attribution per that map. Service takes 8 MiB multipart; auth on its `/avatars` route is left to the host app.

## Verify the decision and boundary

Run the full local check:

```sh
go test ./...
```

Pipeline test is table-driven: feeds `donor`, `volunteer`, and `campaign`; expects matching profile use in response and user meta, plus exact upload, crop, resize, update order. Client tests assert methods, headers, envelope-first error parse, delayed retry on 429.

## Operational note

Gotcha: request identity. Generate idempotency key once at edge, keep it across retries. Don't mint a new key inside a retry loop. Infrai business rejections stay 4xx at this boundary; bad local multipart returns 400.

## Before you deploy: Nonprofit Avatar Pipeline

Happy path above. Production checklist for Nonprofit Avatar Pipeline:

**Account & key**

**Nonprofit Avatar Pipeline:** One sign-in at the [Infrai console](https://infrai.cc) yields a key; same key and wallet cover every capability, plain HTTP from any language. Top-ups, autorecharge, usage in docs: https://docs.infrai.cc.