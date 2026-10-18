# Watterm

A personal WhatsApp companion in the terminal. It links as one device on your account, the same way WhatsApp Web does.

Requires Go 1.26.5 or newer.

## Build

```powershell
go build -o watterm.exe ./cmd/watterm
```

## Run

```powershell
go run ./cmd/watterm
```

Or, after building:

```powershell
.\watterm.exe
```

The first launch shows a QR code. In WhatsApp on your phone, open **Linked devices** and scan it. Later launches reuse that session.

Quit with Ctrl+C. A second copy exits immediately, because only one process can use the session.

Session, transcript, and images are stored in `%AppData%\Watterm`.
