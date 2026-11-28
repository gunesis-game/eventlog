# Research: Drive access for several Google accounts sharing one owner's folder

**Status:** Research note, 2026-11-27. It informs D-065, D-069, and the multi-device design D-060; the current specification is owned by [requirements.md](../project/requirements.md) and [architecture.md](../project/architecture.md).

## Question

The log's master store is one Google Drive folder owned by one account. Devices sign in with their own Google accounts. Can each account read the other accounts' files in that folder, and which OAuth permission and app status does that require on a free account?

## Feasibility check (2026-11-27)

A throwaway program (since removed) signed in with Google's desktop Picker flow: the ordinary OAuth sign-in URL with `trigger_onepick=true`, `allow_folder_selection=true`, and `prompt=consent`, requesting only `https://www.googleapis.com/auth/drive.file` (the desktop Picker allows no other scope alongside it), with a loopback redirect `http://127.0.0.1:8765/` and a Desktop-app OAuth client. The redirect returns `picked_file_ids` with the authorization code. The program then looked up the owner's folder, listed it, read another account's file in it, and created a file of its own.

| Run | Result |
|---|---|
| Owner, no Picker (first, web-Picker attempt) | Folder lookup: HTTP 403 "insufficient authentication scopes"; `drive.file` cannot see a folder the app did not create, even for its owner |
| Owner, picked a file inside the folder | Folder lookup 200; listing returned all three of the owner's files; create 200 |
| Second account, picked a file in its own Drive | Folder lookup 404; listing 200 but empty; **create 200**, file owned by the second account |
| Second account, picked the shared folder | Folder lookup 200; listing showed only the second account's own file, **not the owner's files**; create 200 |

Findings:

- With `drive.file`, an Editor can create files in a shared folder knowing only its ID, even without a grant.
- Picking a shared folder grants access to the folder itself, but not to files other accounts own inside it.
- Each account sees only the files its own app instance created, so separate accounts cannot read each other's logs with `drive.file`.
- Setup lessons: the web Picker embedded in a local page failed with a Google 403 page with a Desktop-app client, while the desktop Picker flow needs no API key or project number; a loopback redirect to the root path on a fixed port worked; accounts not listed as test users are refused with `access_denied`.

## Ownership transfer check (2026-11-27)

A second throwaway program (since removed) tested whether files a second account writes into the owner's folder can be handed to the owner automatically, with both accounts signed in with the full `drive` scope. It found or created a tagged folder owned by the owner and shared it with the second account as Editor, then ran four steps, each with its own browser sign-in:

| Step | Account | Result |
|---|---|---|
| Setup | Owner | Folder created, owned by the owner; second account added as Editor |
| Offer | Second | File created in the folder, owned by the second account. Creating a `pendingOwner` permission for the owner returned the owner's existing writer permission, inherited from the folder, unchanged (`pendingOwner: false`); updating that permission with `pendingOwner: true` succeeded |
| Accept | Owner | Listing the folder found the pending offer; updating the owner's permission to `role: owner` with `transferOwnership=true` returned HTTP 200. The owner now owns the file; the second account became a writer |
| Verify | Second | Owner shown as the owner; capabilities `canModifyContent: true`, `canTrash: false`, `canDelete: false`, `canShare: true`; updating the content succeeded; moving it to the trash failed with HTTP 403 `insufficientFilePermissions` |

Findings:

- Ownership can be handed to the owner entirely through the API: the writing account offers, and the owner accepts, with no one acting in Drive or in an email.
- The offer must update the owner's inherited permission on the file; creating a new permission silently returns the existing one.
- After the transfer the writing account can still update the file, as a device appending to its segments needs, and can no longer trash or delete it.
- Editors keep `canShare` by default; the owner can turn this off on the folder.
- Drive reports email addresses with their original capitals (`MarchuanV@...`), so comparisons must ignore case.

## Prior project: pulsyflux-queue

The owner's earlier project, `gunesis-game/pulsyflux-queue` (a browser-only job queue in an owner's Drive folder), used the same multi-account model and reached the same conclusion:

- **Hard reference:** the owner's email is fixed in configuration. The folder is found by a tag and ownership rather than by ID: a Drive search for `appProperties has { key='pfq' and value='root' }` and `'<owner>' in owners`, with the oldest winning if two were created at once. Found IDs are cached locally, so later starts cost one cheap lookup instead of a search.
- **Access:** Drive sharing controls it. Editors use the queue, Viewers get a read-only view, and anyone else gets a "the folder isn't shared with you" error. Only the owner may create the folder.
- **Permission:** the full `drive` scope. Its configuration records why: `drive.file` only covers files the app created for the same person.
- **App status:** kept in Testing with every user listed as a test user, to avoid Google's verification and security assessment.
- **Sign-in:** browser-only, with no refresh tokens. Access tokens (about an hour) are renewed by a silent redirect to Google that reuses the browser's Google session.
- **Updates:** polling Drive every 10 seconds.

## Constraint for eventlog

eventlog is a Go process, often without a person at a browser, so it needs refresh tokens. In Testing status, Google expires a test user's authorization, refresh token included, seven days after consent. Publishing an app that uses the restricted `drive` scope requires Google's restricted-scope verification with a security assessment repeated every 12 months.

## Conclusion

Separate accounts sharing one owner's folder need the restricted `drive` scope. On a free, unverified app this means Testing status, every user listed as a test user (up to 100), and a browser re-approval every seven days per device; publishing later requires Google's verification. The pulsyflux-queue approach to the hard reference (owner email plus a tagged folder found by ownership, IDs cached), the Viewer read-only mode, and the explicit "not shared" rejection carry over.

## Emulator options (2026-11-28)

Before relying on eventlog's own emulator (`internal/googletest`), a search looked for an existing Drive emulator:

- Google publishes none. Its emulators cover Bigtable, Datastore, Firestore, Pub/Sub, and Spanner (`gcloud beta emulators`), and, in the Firebase Local Emulator Suite, Firestore, Realtime Database, Cloud Storage for Firebase, Authentication, Hosting, Cloud Functions, Pub/Sub, and Extensions. The Firebase suite was a candidate for testing the security rules; D-078 later chose to model the rules in eventlog's emulator instead.
- Third-party Drive emulators exist but lack what eventlog depends on. vercel-labs/emulate (Node, in memory) has OAuth and basic file calls but documents no sharing, ownership transfer, owner or appProperties queries, or ranged reads. gfeh-drive-emulator (Rust, in memory, AGPL) has queries and uploads but no OAuth, no permissions, and refuses owner and appProperties queries. pubkey/google-drive-mock (Node) documents only create and get.

Decision (user, 2026-11-28): keep eventlog's own emulator. The behaviours its security rests on (per-account visibility, inherited roles, ownership handover, owner-only trash, the tag and owner search) are modelled from the checks above, its state is on a real file system, and it counts calls and bytes for the benchmarks, in Go with the standard library only. Where it models behaviour without an observation, the package documentation lists it for checking in the manual runs.

## Sources

- [Google: Integrate the Google Picker into desktop and mobile apps](https://developers.google.com/workspace/drive/picker/guides/desktop-mobile-picker)
- [Google: Integrate the Google Picker into web apps](https://developers.google.com/workspace/drive/picker/guides/web-picker?hl=en)
- [Google: Choose Google Drive API scopes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth)
- [Google: Restricted scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/restricted-scope-verification)
- [Google Cloud: Manage app audience (seven-day expiry in Testing)](https://support.google.com/cloud/answer/15549945?authuser=7)
- [Google: OAuth production readiness overview](https://developers.google.com/identity/protocols/oauth2/production-readiness/overview)
- [Google Drive help: storage for files in shared folders](https://support.google.com/drive/answer/17196458)
- [Google Developer forums: Picker 403 inside an iframe](https://discuss.google.dev/t/creating-google-drive-picker-append-into-iframe-to-my-custom-div-use-touri-show-error-403-you-do/91949)
- [vercel-labs/emulate](https://github.com/vercel-labs/emulate)
- [gfeh-drive-emulator](https://docs.rs/gfeh-drive-emulator/latest/gfeh_drive_emulator/)
- [pubkey/google-drive-mock](https://github.com/pubkey/google-drive-mock)
- [gcloud beta emulators](https://pbrisbin.github.io/bluebook/man1/gcloud_beta_emulators.1.html)
- [Firebase Local Emulator Suite](https://firebase.google.com/docs/emulator-suite)
