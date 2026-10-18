# Separate Transcript store

The Companion keeps a Session so it stays linked, and a Transcript so Chats and Messages remain after a restart. We decided these live apart: a Session database, a Transcript database, and a media folder, so a whatsmeow upgrade cannot rewrite the Transcript. The Transcript is plaintext, and it survives logout, because logout ends the Session and does not erase the record.

## Consequences

- View-once images and unsupported media are never downloaded.
- Disappearing Messages and revoke Tombstones are enforced.
- Logs must not contain Message text or Session material.
