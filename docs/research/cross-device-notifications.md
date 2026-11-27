# Research: cross-device change notifications on a free Google account

**Status:** Research note, 2026-11-27. It informs D-061 to D-064 and the proposed multi-device design D-060; the current specification is owned by [requirements.md](../project/requirements.md) and [architecture.md](../project/architecture.md). Some claims come from secondary sources (third-party guides, forums, directories) and are marked as such.

## Question

Devices share one log in Google Drive (D-056). When one device commits, the others should learn of it promptly, without polling. Most devices sit behind home routers or carrier-grade NAT and cannot accept incoming connections, and the deployment must stay on free tiers and the Go standard library. What can deliver those notifications?

## Constraints established

### Google Drive and service accounts

- Since April 2025, service accounts have no Drive storage quota and cannot own files. They can upload only into shared drives (a paid Workspace feature) or act for a person through OAuth. One integration project reports that service accounts created before April 2025 keep their quota; Google's own documentation states the rule without that exception.
- On a free account, eventlog must therefore sign in as the user with OAuth, approving access once per device in a browser.

### Drive push notifications

- `files.watch` and `changes.watch` deliver to a webhook URL that must be public HTTPS with a valid certificate; self-signed and untrusted certificates are rejected.
- Notifications carry no content: a receiver must then fetch what changed.
- Third-party guides report that watch channels expire (about a day for single files, up to a week for the changes feed) and must be renewed; this was not confirmed in Google's reference.
- Notifications do not count against quota; calls to set up and stop channels do.
- Google's pages disagree about Drive API pricing: one says use is at no additional cost, a newer one says exceeding daily no-charge allowances is planned to incur charges later in 2026.

### Google Workspace Events API

- Drive events can be published to a Google Cloud Pub/Sub topic.
- Google's announcement says Drive events are available to Google Workspace customers and Workspace Individual subscribers, so a free consumer account is not eligible. Drive support was added as a Developer Preview in July 2025.
- Receiving from Pub/Sub without a public endpoint needs StreamingPull, which is gRPC only; the REST `pull` method is request-and-wait.
- Pub/Sub's first 10 GiB of basic delivery per month is free, but that does not help without eligible events.

### NAT

- Two devices behind NAT cannot find each other without a third party both can reach.
- UPnP IGD, NAT-PMP, and PCP let a program open a port on the home router, but fail behind carrier-grade NAT, where there is no public address to map, and are often disabled.

## Options examined

### WebRTC, ICE, STUN, TURN

- STUN discovers a device's public address, but that address only accepts packets from a peer just contacted, and only briefly; hole punching needs both peers to coordinate first.
- WebRTC deliberately leaves signalling (the exchange of offers, answers, and candidate addresses) to the application: RFC 9429 (JSEP) leaves signalling-plane behaviour to the application, MDN states signalling is not defined in the specification, and web.dev notes an intermediary server is needed. RFC 9725 (WHIP) likewise notes WebRTC does not specify a signalling transport.
- "Serverless" WebRTC moves signalling to a human (copy and paste, QR codes) or another existing channel.
- Symmetric and carrier-grade NAT defeat hole punching; only TURN, a relay server and usually not free, works there.
- A full WebRTC stack (ICE, DTLS, SCTP) is not practical with the standard library; it needs a large library such as Pion.
- A peer-to-peer mesh needs N×(N−1)/2 connections for N devices.
- Conclusion: WebRTC still needs a reachable rendezvous and adds a large dependency, to carry notifications of a few dozen bytes. Drive already carries the data, so peer-to-peer brings no benefit.

### Self-hosted relay

- A relay receives "I committed" notes from devices and pushes them over each device's single outbound connection. Devices then need no incoming address.
- The relay itself must be reachable: a public server, a router port forward, or a tunnel. That fails the "run everything from a local machine" case.

### Centrifugo

- Centrifugo (centrifugal.dev) is a self-hosted real-time messaging server, offering unidirectional SSE (`/connection/uni_sse`), HTTP-streaming, WebSocket, gRPC, and WebTransport, plus an HTTP publish API (`/api/publish`, API-key authenticated).
- It scales to tens of thousands of connections per node and clusters with Redis, KeyDB, NATS, Tarantool, or PostgreSQL.
- The open-source core is free to self-host (a third-party directory states Apache 2.0; not confirmed against the repository). Centrifugo PRO is commercial, sold only to businesses, with a key-less sandbox of 20 connections.
- No official hosted free tier was found.
- It is a production-grade self-hosted relay, so it inherits the relay's reachability requirement.

### Ably

- Ably is hosted, so devices connect outbound and NAT is not an issue. It offers a read-only SSE endpoint (channels named in the request, API key as `key=` or HTTP Basic) and REST publishing (`POST /channels/{name}/messages`).
- The user's free account allows 200 concurrent connections, 6 million messages per month, 500 messages per second per account, 50 per second per channel, 64 KiB messages, and 24-hour history.
- Ably's pages disagree on the free figures (one says 500 connections and channels). One page says usage over the free limits is throttled rather than cut off.
- Ably accepts only its own API keys, or tokens and JWTs signed with that key. A Google token cannot be used directly; a server would have to verify it and mint an Ably token, reintroducing a reachable server, or every device would hold the API key.
- How Ably counts delivered copies of a message against the monthly allowance was not verified.

### Firebase Realtime Database

- **Streaming:** REST endpoints support server-sent events: a client sends `Accept: text/event-stream` and receives `put` and `patch` events, plus `keep-alive`, `cancel`, and `auth_revoked`, for changes at that location. On connecting, the stream first delivers the current data.
- **Coupled to the database:** the stream reports state changes at database locations, not messages; notifying means writing. Rapid updates may arrive as only the latest value, and there is no queue or history beyond the current value.
- **Free (Spark) plan limits:**
  - 100 simultaneous connections, not raisable on Spark (200,000 per database on the paid Blaze plan). Server-sent-events connections count; plain REST requests do not.
  - 1 GB stored and 10 GB downloaded per month.
  - On Spark, exceeding a limit turns the database off for the rest of the month instead of billing.
- **Authentication with Google sign-in:**
  - The REST API documents Google OAuth2 access tokens for service accounts. Their use with a personal account's token is undocumented, and such tokens bypass security rules.
  - The documented user route is Firebase Authentication: exchange a Google ID token through `accounts:signInWithIdp` (`providerId=google.com`, `id_token=` for Google) for a Firebase ID token and refresh token, then pass the Firebase ID token as `auth=` on database requests, which apply the database's security rules. Refresh tokens are exchanged for new ID tokens at `securetoken.googleapis.com`.
  - Firebase Authentication on Spark is no-cost up to 50,000 monthly active users, with 1,600 sign-ins per minute. Firebase's limits and pricing pages describe the tiers somewhat differently.

## Comparison

| | NAT-proof | Server to run | Free scale | Google sign-in | Standard library only |
|---|---|---|---|---|---|
| Drive webhooks | No: every receiver needs public HTTPS | Each device | Free | Yes | Yes |
| Workspace Events + Pub/Sub | Yes, with StreamingPull | No | Not available to free accounts | Yes | No (gRPC) |
| WebRTC | Partly: needs signalling, plus TURN on hard NATs | Signalling, TURN | — | — | No |
| Self-hosted relay or Centrifugo | Only if the relay is reachable | Yes | You host it | Optional | Yes |
| Ably | Yes | No | 200 online devices, 6M messages per month | No (own keys) | Yes |
| Firebase Realtime Database | Yes | No | 100 online devices, 10 GB per month | Yes, via Firebase Authentication | Yes |

## Conclusion

Firebase Realtime Database, used as a "latest position per device" board streamed over SSE and authenticated through Firebase Authentication with the user's Google sign-in, is the only option found that is NAT-proof, needs no server, stays free at small scale, keeps Google as the single identity, and needs only the standard library. Its limits (100 online devices; 10 GB of downloads per month, with a monthly cutoff when exceeded) bound free-tier use; larger fleets need the paid plan or a self-hosted relay.

## Sources

- [Google: Drive API, resolve errors (service account storage)](https://developers.google.com/workspace/drive/api/guides/handle-errors)
- [Google: Shared drives overview](https://developers.google.com/workspace/drive/api/guides/about-shareddrives)
- [n8n pull request: service accounts and shared drives](https://github.com/n8n-io/n8n/pull/18952)
- [Google Developer forums: 403 storageQuotaExceeded](https://discuss.google.dev/t/error-403-storagequotaexceeded-when-the-service-accounts-drive-is-completely-empty/194265)
- [Google: Drive push notifications](https://developers.google.com/workspace/drive/api/guides/push?hl=en)
- [Google: changes.watch reference](https://developers.google.com/workspace/drive/api/reference/rest/v3/changes/watch)
- [Google: Drive API usage limits](https://developers.google.com/workspace/drive/api/guides/limits)
- [Google: Drive API v3 limits (older page)](https://developers.google.com/drive/api/v3/limits)
- [Hookdeck: Guide to Google Workspace webhooks](https://hookdeck.com/webhooks/platforms/guide-to-google-workspace-webhooks-features-and-best-practices)
- [Fast.io: Setting up Google Drive webhooks](https://fast.io/resources/google-drive-webhook/)
- [Google: Work with events from Google Drive](https://developers.google.com/workspace/drive/api/guides/events-overview)
- [Google Workspace Updates: Drive Events API in Developer Public Preview](https://workspaceupdates.googleblog.com/2025/07/google-drive-events-api-now-available.html)
- [Google: Workspace Events API release notes](https://developers.google.com/workspace/events/release-notes)
- [Google Cloud: Receive messages from a pull subscription](https://docs.cloud.google.com/pubsub/docs/pull-messages)
- [Google Cloud: Pub/Sub pricing](https://cloud.google.com/pubsub/pricing)
- [RFC 6970: UPnP IGD and PCP interworking](https://www.rfc-editor.org/rfc/rfc6970)
- [pfSense: UPnP IGD and PCP](https://docs.netgate.com/pfsense/en/latest/book/services/upnp-and-nat-pmp.html)
- [OpenWrt forum: CGNAT and PCP](https://forum.openwrt.org/t/help-with-cgnat-and-pcp/195382)
- [RFC 8825: WebRTC Overview](https://www.rfc-editor.org/rfc/rfc8825.html)
- [RFC 9429: JavaScript Session Establishment Protocol (JSEP)](https://www.rfc-editor.org/rfc/rfc9429.html)
- [RFC 9725: WebRTC-HTTP Ingestion Protocol (WHIP)](https://datatracker.ietf.org/doc/html/rfc9725)
- [MDN: Lifetime of a WebRTC session](https://developer.mozilla.org/docs/Web/API/WebRTC_API/Session_lifetime)
- [web.dev: Build the backend services needed for a WebRTC app](https://web.dev/articles/webrtc-infrastructure)
- [RxDB: Google Drive replication (WebRTC signalling)](https://rxdb.info/replication-google-drive.html)
- [Centrifugo: official site](https://centrifugal.dev)
- [Centrifugo: unidirectional SSE](https://centrifugal.dev/docs/transports/uni_sse)
- [Centrifugo: server API](https://centrifugal.dev/docs/server/server_api)
- [Centrifugo PRO overview](https://centrifugal.dev/docs/pro/overview)
- [Centrifugo: FAQ](https://centrifugal.dev/components/FAQ)
- [Toolradar: Centrifugo pricing (licence claim)](https://toolradar.com/tools/centrifugo/pricing)
- [Ably: SSE and raw HTTP streaming API](https://ably.com/docs/api/sse)
- [Ably: REST API reference](https://ably.com/docs/rest-api)
- [Ably: Token auth](https://ably.com:443/docs/auth/token)
- [Ably: JSON Web Tokens](https://ably.com/docs/auth/token/jwt)
- [Ably: Free package](https://ably.com/docs/platform/pricing/free)
- [Ably: Free package (older page)](https://ably.com/docs/pricing/free)
- [Ably: Pricing FAQs](https://ably.com/docs/pricing/faqs)
- [Firebase: Retrieving data, streaming from the REST API](https://firebase.google.com/docs/database/rest/retrieve-data)
- [Firebase: Database REST API reference](https://firebase.google.com/docs/reference/rest/database/)
- [Firebase blog: Announcing streaming for the REST API](https://firebase.blog/posts/2014/03/announcing-streaming-for-firebase-rest)
- [Firebase: Realtime Database limits](https://firebase.google.com/docs/database/usage/limits?hl=en)
- [Firebase: Realtime Database FAQ and troubleshooting](https://firebase.google.com/docs/database/faq-and-troubleshooting?hl=en)
- [Firebase: Authenticate REST requests](https://firebase.google.com/docs/database/rest/auth)
- [Firebase: Authenticate REST requests with user ID tokens](https://firebase.google.com/docs/reference/rest/database/user-auth?authuser=0)
- [Google Groups: Google OAuth2 tokens versus ID tokens](https://groups.google.com/forum/?_escaped_fragment_=topic%2Ffirebase-talk%2FqcAqulTpZaM)
- [Daily Developer Blog: Firebase REST 401 for everyone but the owner](https://dailydevsblog.com/troubleshoot/resolved-firebase-rest-api-401-unauthorized-for-everyone-but-me-182057/)
- [Firebase: Auth REST API](https://firebase.google.cn/docs/reference/rest/auth)
- [Identity Platform: accounts.signInWithIdp](https://docs.cloud.google.com/identity-platform/docs/reference/rest/v1/accounts/signInWithIdp)
- [Google Groups: id_token versus access_token for signInWithIdp](https://groups.google.com/g/firebase-talk/c/CiZhi0K7YLY)
- [Firebase: Authentication limits](https://firebase.google.com/docs/auth/limits)
