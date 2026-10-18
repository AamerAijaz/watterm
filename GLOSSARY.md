# Watterm

The language for a personal Companion linked to one WhatsApp Account.

## Language

### The link

**Account**:
The single WhatsApp identity the Companion is linked as.
_Avoid_: User, login, phone

**Companion**:
Watterm acting as one linked device on the Account.
_Avoid_: Client, bot, CLI

**Session**:
The credentials that keep the Companion linked to the Account.
_Avoid_: Login, token, cookie

### Chats

**Chat**:
One exchange identified by WhatsApp's address for it, either a Direct chat or a Group. The address is the identity, not the Display name.
_Avoid_: Conversation, thread, room

**Direct chat**:
A Chat between the Account and one other person, however many addresses WhatsApp holds for that person.
_Avoid_: DM, private chat, one-to-one

**Group**:
A Chat among many Participants, with a subject.
_Avoid_: Room, channel, community

**Display name**:
The label shown for a Chat or a Participant. It is not a Chat's identity.
_Avoid_: Username, nickname, contact

**Participant**:
A person in a Group, labeled by a Display name.
_Avoid_: Member, user, contact

### The record

**Message**:
One item in a Chat: text, an Image, a Marker, or a Tombstone.
_Avoid_: Notification, event

**Image**:
A picture Message other than a View-once image.
_Avoid_: Photo, media, attachment

**View-once image**:
A picture the sender marked to be seen once. It is not an Image.
_Avoid_: Disappearing message, secret photo

**Marker**:
A Message that names a kind of content and does not carry that content.
_Avoid_: Placeholder, stub

**Tombstone**:
The trace that remains after a Message's content is gone.
_Avoid_: Placeholder, deleted message

**Transcript**:
The Companion's own record of Chats and Messages. It is a partial record, not the Account's full history.
_Avoid_: History, log, database, archive

**Receipt**:
The outgoing state of a Message: pending, sent, delivered, read, or failed.
_Avoid_: Tick, acknowledgement, status

**Unread**:
The Messages in a Chat that the Account has not opened in the Companion.
_Avoid_: Badge, notification
