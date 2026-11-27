# Drive access check

A throwaway feasibility check, not part of the eventlog library. Delete it once its result is recorded.

## Question

Can a second Google account, signed in with only the non-sensitive `drive.file` permission, use a folder that another account owns and shared with it, after picking that folder once in Google Picker? If so, eventlog can keep a fixed reference to one Drive folder and admit any account the owner shares it with, with no restricted permission (and no yearly security assessment) and no weekly re-sign-in once the OAuth app is published.

## What it does

1. Signs in through the browser as whichever account you choose, asking for `openid email drive.file` only.
2. **Step 1:** looks up the shared folder before picking it. The expected result is HTTP 404, because `drive.file` cannot see a folder the app did not create.
3. Opens Google Picker so you can pick the shared folder.
4. **Step 2:** looks up the folder again.
5. **Step 3:** lists the folder and reads one file owned by another account.
6. **Step 4:** creates a small text file of its own in the folder.

It prints a report to the console and the browser. Steps 2 to 4 succeeding means the model works.

## One-time setup (owner account)

In the [Google Cloud console](https://console.cloud.google.com/), in the project that will hold eventlog's OAuth client:

1. **APIs:** enable the **Google Drive API** and the **Google Picker API**.
2. **OAuth consent screen / Audience:** user type **External**, publishing status **Testing**, and add the second account as a **test user**. Only listed test users can sign in while the app is in Testing.
3. **OAuth client:** create an OAuth client ID of type **Desktop app**, and download its JSON as `oauth-client.json`.
4. **API key:** create an API key and restrict it to the **Google Picker API**.
5. **Project number:** copy it from the project's dashboard. Picker uses it as the app ID, which is what lets a pick grant `drive.file` access.

In Google Drive, signed in as the owner:

6. Create a folder, put any small text file in it, and share the folder with the second account as **Editor**.
7. Copy the folder ID from the folder's address: `drive.google.com/drive/folders/<FOLDER_ID>`.

## Run (as the second account)

From the repository root:

```powershell
go run ./docs/research/drive-access-check -client oauth-client.json -apikey <API_KEY> -appid <PROJECT_NUMBER> -folder <FOLDER_ID>
```

Sign in with the **second** account in the browser, approve, pick the shared folder in Picker, and wait for the report. Optionally run it a second time as the owner for comparison.

The program keeps the tokens in memory only and writes nothing to disk. It does create one small `drive-access-check-<time>.txt` file in the shared folder, which you can delete afterwards.

## Reading the result

| Outcome | Meaning |
|---|---|
| Step 1 HTTP 404, steps 2–4 HTTP 200 | The fixed-folder model works with `drive.file` and Picker |
| Step 2 HTTP 200, but step 3 shows no other account's files or step 3's read fails | Picking grants the folder itself but not other accounts' files in it; the model needs the restricted `drive` permission |
| Step 2 HTTP 404 | Picking did not grant access with this setup; check that the app ID is the project number and that the API key allows the Picker API |
| Sign-in shows "access blocked" | The account is not listed as a test user |
