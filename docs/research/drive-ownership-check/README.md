# Drive ownership check

A throwaway feasibility check, not part of the eventlog library. Delete it once its result is recorded.

## Question

When a second Google account creates a file in a folder the owner shared with it, can the file's ownership be handed to the owner automatically through the Drive API? The second account can only offer ownership (a pending owner); the owner has to accept. If an owner's device can accept through the API, the Drive backend can make the owner own every file in the master store. The check also shows whether the second account can still update the file afterwards, which its device needs in order to keep appending, and whether it can still move the file to the trash.

## Setup

The Desktop-app `oauth-client.json` in the repository root (never committed), and both accounts listed as test users. The check asks for the full `drive` permission. Nothing else needs preparing: the check finds or creates its own folder and file.

- **Folder:** `eventlog-ownership-check`, found as eventlog finds its folder, by an `appProperties` tag and the owner's ownership (oldest wins). The setup step creates it only if it does not exist, and shares it with the second account as Editor only if it is not shared yet.
- **File:** `ownership-check.txt` in that folder. The offer step creates it only if it does not exist.

## Run

From the repository root, four steps, each opening the browser to sign in. Pick the account named for each step. Every step can be run again safely.

1. **Setup, as the owner:**

   ```powershell
   go run ./docs/research/drive-ownership-check -step setup -owner <OWNER_EMAIL> -member <SECOND_EMAIL>
   ```

2. **Offer, as the second account:** finds or creates the file and offers its ownership to the owner. Google may email the owner about the offer.

   ```powershell
   go run ./docs/research/drive-ownership-check -step offer -owner <OWNER_EMAIL>
   ```

3. **Accept, as the owner:** accepts every pending ownership offer in the folder, as an owner's device would at start.

   ```powershell
   go run ./docs/research/drive-ownership-check -step accept -owner <OWNER_EMAIL>
   ```

4. **Verify, as the second account:** reports the file's owner and the second account's capabilities, updates the file's content, and tries to move it to the trash, restoring it if that works. It never deletes the file permanently.

   ```powershell
   go run ./docs/research/drive-ownership-check -step verify -owner <OWNER_EMAIL>
   ```

The program keeps tokens in memory only and writes nothing to disk. Delete the `eventlog-ownership-check` folder afterwards.

## Reading the result

| Outcome | Meaning |
|---|---|
| The offer succeeds with `pendingOwner: true`; the accept step returns HTTP 200 with `role: owner` | Ownership can be handed to the owner automatically |
| The offer fails, and the retry fails too | A second account cannot offer ownership to the owner through the API |
| The accept step fails | The owner cannot accept through the API; acceptance would need the owner by hand |
| Verify step 1 shows the owner as owner; step 2 HTTP 200 | After the transfer the second account can still append, as its device needs |
| Verify step 3 HTTP 403 | After the transfer the second account can no longer move the file to the trash |
