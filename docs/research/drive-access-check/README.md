# Drive access check

A throwaway feasibility check, not part of the eventlog library. Delete it once its result is recorded.

## Question

Can a second Google account, signed in with only the non-sensitive `drive.file` permission, use a folder that another account owns and shared with it, after picking that folder once in Google Picker? If so, eventlog can keep a fixed reference to one Drive folder and admit any account the owner shares it with, with no restricted permission (and no yearly security assessment) and no weekly re-sign-in once the OAuth app is published.

## What it does

It uses Google's desktop Picker flow: the ordinary sign-in URL with `trigger_onepick=true` and `allow_folder_selection=true`, asking only for `drive.file` (the desktop Picker allows no other scope alongside it).

1. Opens the browser to sign in. On the consent page, Google shows Picker: pick the shared folder there.
2. **Step 1:** reports the picked IDs returned with the sign-in, and whether the shared folder is among them.
3. **Step 2:** looks up the folder.
4. **Step 3:** lists the folder and reads one file owned by another account.
5. **Step 4:** creates a small text file of its own in the folder.

It prints a report to the console and the browser. Steps 2 to 4 succeeding means the model works. Without picking, `drive.file` cannot see the folder at all: an earlier run without Picker got HTTP 403 "insufficient authentication scopes" for it, even as the owner.

## One-time setup (owner account)

In the [Google Cloud console](https://console.cloud.google.com/), in the project that will hold eventlog's OAuth client:

1. **APIs:** enable the **Google Drive API** and the **Google Picker API**.
2. **OAuth consent screen / Audience:** user type **External**, publishing status **Testing**, and add every account that will run the check as a **test user**. Only listed test users can sign in while the app is in Testing.
3. **OAuth client:** create an OAuth client ID of type **Desktop app**, and download its JSON as `oauth-client.json`.

No API key or project number is needed for the desktop Picker flow.

In Google Drive, signed in as the owner:

4. Create a folder, put any small text file in it, and share the folder with the second account as **Editor**.
5. Copy the folder ID from the folder's address: `drive.google.com/drive/folders/<FOLDER_ID>`.

## Run (as the second account)

From the repository root:

```powershell
go run ./docs/research/drive-access-check -client <PATH_TO>/oauth-client.json -folder <FOLDER_ID>
```

The browser returns to `http://127.0.0.1:8765/` after sign-in; add `-port <N>` to use another port. Sign in with the **second** account, approve, pick the shared folder when Picker appears, and wait for the report. Optionally run it a second time as the owner for comparison.

The program keeps the tokens in memory only and writes nothing to disk. It does create one small `drive-access-check-<time>.txt` file in the shared folder, which you can delete afterwards.

## Reading the result

| Outcome | Meaning |
|---|---|
| Step 1 shows the folder picked, steps 2–4 HTTP 200, and step 3 reads the other account's file | The fixed-folder model works with `drive.file` and Picker |
| Steps 2 and 4 HTTP 200, but step 3 shows no other account's files or its read fails | Picking grants the folder itself but not other accounts' files in it; the model needs the restricted `drive` permission |
| Step 1 shows no picked IDs, or step 2 HTTP 403 or 404 | Picker did not run or did not grant the folder; check that the Picker API is enabled in the same project as the OAuth client |
| Sign-in shows "access blocked" or `access_denied` | The account is not listed as a test user |
