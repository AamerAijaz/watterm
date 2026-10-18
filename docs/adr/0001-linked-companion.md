# One linked Companion

Watterm is a personal terminal for one WhatsApp Account. We decided it is one linked Companion for that one Account, running as one foreground process, speaking through whatsmeow. The Business Cloud API addresses a business rather than this Account's chats, and a background daemon would keep the link alive after the process quits.

## Considered options

- Business Cloud API: rejected. It requires a business, and it does not carry this Account's personal chats.
- Background daemon: rejected. The Companion is the foreground process. Live delivery exists only while that process is open.
