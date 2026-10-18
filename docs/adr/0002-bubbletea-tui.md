# Bubbletea for the TUI

The Companion's screen moves through link modes, such as showing a QR, reconnecting, an open Chat, or logged out, while events arrive from the link. We decided the TUI is bubbletea, because those modes and events are one fold over messages. tview would supply the panes sooner, and every event would have to be marshalled onto the UI thread.

## Considered options

- tview: rejected. The panes arrive faster, and the event loop fights the link.
