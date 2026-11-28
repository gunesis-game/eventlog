// Package googletest is a test-only, in-process emulator of the Google
// services eventlog's web storage calls: OAuth sign-in and token refresh, and
// the Drive v3 file and permission calls, and Firebase Authentication and
// the Realtime Database that carry change notifications. Production code
// runs against it
// unchanged, with only its service addresses pointed at the emulator, so
// tests reach all of that code and never spend Google quota. No automated
// test, benchmark, or CI job calls real Google services.
//
// The emulator serves Google's own paths on one local address:
//
//	GET  /o/oauth2/v2/auth                 authorization (the browser step)
//	POST /token                            code exchange and token refresh
//	     /drive/v3/files...                file metadata, listing, ranged reads
//	     /upload/drive/v3/files...         file creation and content uploads
//	     /drive/v3/files/{id}/permissions  sharing and ownership offers
//	POST /v1/accounts:signInWithIdp        Google ID token for a Firebase one
//	POST /v1/token                         Firebase token refresh
//	     /rtdb/{path}.json                 Realtime Database REST and streams
//
// Its behaviour follows what the feasibility checks observed of real Drive
// (docs/research/drive-access-model.md):
//
//   - The creating account owns a file. A file or folder an account has no
//     access to is not found (404). Viewers are refused writes (403).
//   - Sharing roles are inherited from the folders above a file; the owner of
//     a folder holds an inherited writer role on files others create in it.
//   - Creating a permission for an account that already has that access
//     returns the existing permission unchanged. An ownership offer updates
//     the new owner's permission with pendingOwner; the new owner accepts by
//     updating it to role owner with transferOwnership. Afterwards the
//     earlier owner is a writer: it can change the content but cannot trash
//     the file (403 insufficientFilePermissions).
//   - Folders are found by appProperties and '<owner>' in owners queries;
//     owner email addresses compare without regard to case.
//   - Content is read in byte ranges and always uploaded whole; there is no
//     append. Trashed files stay readable by ID and are listed unless the
//     query asks for trashed = false.
//   - The app is in Testing: only listed test users may sign in, and a
//     refresh token expires seven days after consent. Access tokens last an
//     hour. Every Drive call needs the full drive scope.
//
// Firebase follows its REST documentation:
//
//   - A sign-in with the openid scope returns a Google ID token, which
//     signInWithIdp exchanges for a Firebase ID token and refresh token in
//     the owner's project, recording the account there on its first
//     exchange. ID tokens last an hour; Firebase refresh tokens do not
//     expire.
//   - The database answers GET, PUT, PATCH (children, with paths as keys),
//     and DELETE of a location's JSON; it keeps no nulls or empty objects.
//     Refusals are 401 with {"error": "..."}: an expired or unparseable
//     token, or the security rules.
//   - A GET with Accept: text/event-stream is redirected once (307), then
//     streams: first a put of the whole value at "/", then a put for each
//     changed value and a patch for each PATCH, with paths relative to the
//     location, and keep-alive while idle.
//   - The security rules of D-066 are enforced once the owner installs them;
//     until then the database refuses everything, as in locked mode.
//
// Some behaviour is modelled without an observation behind it, and is to be
// checked against real Drive when the manual runs of 12.8 and 12.9 happen:
// only a file's owner may trash it, whoever owns its folder; ownership moves
// only by the pending owner accepting an offer; readers cannot list
// permissions; and the default fields of each response. For Firebase: a
// stream whose sign-in expires sends auth_revoked and ends, and one whose
// read the rules no longer allow sends cancel with null data and ends; a
// connection beyond the free plan's 100, and any request while the
// database is switched off, gets 503; keep-alive comes every 30 seconds;
// and ID tokens are signed by the emulator's own key rather than Google's.
//
// The emulator keeps its state on a real file system, in a folder it is
// given: each Drive file's content as a file named by its file ID, and all
// metadata, accounts, and tokens in a metadata file beside them, and the
// Realtime Database's data in a JSON file of its own, so large
// files are stored as they would be, an emulator restarted over the same
// folder keeps its contents, and a failing test leaves inspectable files. Its
// own code uses the Go standard library only, so the yardstick shares no code
// with what it measures. Its tests make every request a correct client makes
// through eventlog's service clients (internal/googleauth, internal/drive,
// internal/firebase), which are written from Google's contracts rather than
// the emulator's; where the two differ, the emulator is corrected (D-081).
// WriteClientFile and Browse give tests the client file and the browser
// step.
//
// It counts requests and bytes per account and per call, a stream's bytes
// while it is open, so benchmarks can report Google calls, board writes,
// connections, and streamed bytes per operation, and it can add a delay to
// every call to model network latency.
package googletest
