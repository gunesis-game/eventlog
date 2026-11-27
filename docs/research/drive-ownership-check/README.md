# Drive ownership check

A throwaway feasibility check, not part of the eventlog library. Delete it once its result is recorded.

## Question

When a second Google account creates a file in a folder the owner shared with it, can the file's ownership be handed to the owner automatically through the Drive API? The second account can only offer ownership (a pending owner); the owner has to accept. If an owner's device can accept through the API, the Drive backend can make the owner own every file in the master store. The check also shows whether the second account can still update the file afterwards, which its device needs in order to keep appending, and whether it can still move the file to the trash.

## Setup

The same as the earlier access check: the Desktop-app `oauth-client.json` in the repository root (never committed), both accounts listed as test users, and the shared folder with the second account as **Editor**. The check asks for the full `drive` permission. Copy the folder ID from the folder's address: `drive.google.com/drive/folders/<FOLDER_ID>`.

## Run

From the repository root, three steps, each opening the browser to sign in. Pick the account named for each step.

1. **Offer, as the second account:**

   ```powershell
   go run ./docs/research/drive-ownership-check -step offer -folder <FOLDER_ID> -owner <OWNER_EMAIL>
   ```

   It creates `drive-ownership-check-<time>.txt` in the folder, offers its ownership to the owner, and prints the file ID. Google may email the owner about the offer.

2. **Accept, as the owner:**

   ```powershell
   go run ./docs/research/drive-ownership-check -step accept -folder <FOLDER_ID> -owner <OWNER_EMAIL>
   ```

   It lists the folder and accepts every pending ownership offer to the owner, as an owner's device would at start.

3. **Verify, as the second account:**

   ```powershell
   go run ./docs/research/drive-ownership-check -step verify -file <FILE_ID>
   ```

   It reports the file's owner and the second account's capabilities, updates the file's content, and tries to move it to the trash, restoring it if that works. It never deletes the file permanently.

The program keeps tokens in memory only and writes nothing to disk. Delete the test file from the folder afterwards.

## Reading the result

| Outcome | Meaning |
|---|---|
| Offer step 3 HTTP 200 with `pendingOwner: true`; accept step 2 HTTP 200 with `role: owner` | Ownership can be handed to the owner automatically |
| Offer step 3 fails, and the retry fails too | A second account cannot offer ownership to the owner through the API |
| Accept step 2 fails | The owner cannot accept through the API; acceptance would need the owner by hand |
| Verify step 1 shows the owner as owner; step 2 HTTP 200 | After the transfer the second account can still append, as its device needs |
| Verify step 3 HTTP 403 | After the transfer the second account can no longer move the file to the trash |
