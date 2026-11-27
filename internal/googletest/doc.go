// Package googletest is a test-only, in-process emulator of the Google
// services eventlog's web storage calls: OAuth sign-in and token refresh, and
// the Drive v3 file and permission calls. Production code runs against it
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
// Some behaviour is modelled without an observation behind it, and is to be
// checked against real Drive when the manual runs of 12.8 and 12.9 happen:
// only a file's owner may trash it, whoever owns its folder; ownership moves
// only by the pending owner accepting an offer; readers cannot list
// permissions; and the default fields of each response.
//
// The emulator keeps its state on a real file system, in a folder it is
// given: each Drive file's content as a file named by its file ID, and all
// metadata, accounts, and tokens in a metadata file beside them, so large
// files are stored as they would be, an emulator restarted over the same
// folder keeps its contents, and a failing test leaves inspectable files. It
// uses the Go standard library directly, not eventlog's internal/filesystem,
// so the yardstick shares no code with what it measures.
//
// It counts requests and bytes per account and per call, so benchmarks can
// report Google calls and bytes per operation, and it can add a delay to
// every call to model network latency.
package googletest
