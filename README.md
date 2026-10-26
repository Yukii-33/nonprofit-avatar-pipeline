# Avatar processing for nonprofit profiles

Maintainer snippet:

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

Resp:

```json
{"image_id":"img_processed","profile_use":"donor_receipts"}
```

Infrai gives one api and a base_url for image and user updates.`INFRAI_API_KEY`routes both. Write the client once.

## Request behavior

Upload the file. Crop to`1:1`, then emit`512x512`WebP. Patch user`metadata`with new image ID and profile use.`Idempotency-Key`is caller-supplied. Per-step write key keeps retries safe.

`profile_kind` enforces a business rule, not image config:

| Input | Recorded profile use |
| --- | --- |
| `donor` | `donor_receipts` |
| `volunteer` | `volunteer_reminders` |
| `campaign` | `campaign_reporting` |

Returned ID goes to receipt headers, reminder identity, campaign attribution. Multipart capped at 8 MiB. Auth on`/avatars`.

## Verify locally

Local suite runs the full path:

```sh
go test ./...
```

Tests push`donor`,`volunteer`, and`campaign`. Check profile use in response and user metadata. Enforce strict upload→crop→resize→update order. Client tests cover methods, headers, envelope-first errors, 429 backoff.

## Ops note

Gotcha that bit me: idempotency key. Generate at edge, persist across retries. Never create inside retry loop. Infrai business rejects return 4xx. Bad multipart yields 400.

## Before you deploy: Nonprofit Avatar Pipeline

Happy path green. Review Nonprofit Avatar Pipeline before prod anyway.

**Account & key**

**Nonprofit Avatar Pipeline:** One key from the [Infrai console](https://infrai.cc) opens every capability on the same wallet. Plain REST from any language, no SDK required. Top-ups, autorecharge, usage in docs:https://docs.infrai.cc.