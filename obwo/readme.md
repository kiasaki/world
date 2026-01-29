# obwo

a personal cloud, minimal, encrypted, local-first, synced

apps for: files, notes, passwords, tasks, contacts, calendars, mails

with extra apps deriving from files / web: pictures, music, news, search

(coding style is sorta compressed, you don't have to like it, i like it)

## files

- `main.go` (~300l) few user/sync rpcs and serves static assets
- `index.html` (~600l) styles, markup utils and app code

rest doesn't really matter

- `app.webmanifest` pwa manifest, has to be separate
- `sw.js` service worker, "
- `icon.png` app icon, "
- `font.ttf`
- `makefile` dev, build and deploy w/ scp

## glossary

- username: hash of username for user retrieval
- id: immutable id under which user state is stored
- secret: api key, shared secret for access control
- master: encrypted master key stored on user
- datum: EAVT quadruplet representing database data
- chunk: ~1mb array of datums stored together
- checkpoint: `chunk.index` used by app to track sync position
- blob: ~1mb encrypted, content addressed data stored outside of synced db

## s3 storage paths

- `n/<hashed-username>`: user id as base64 string
- `u/<key>`: user data (salt, master, chunk, secret (not returned))
- `d/<key>/<chunk>`: chunk of datums
- `b/<key>/<hashed-data>`: blob of data

## crypto

- `masterKey` = random(32)
- `salt` = random(32)
- `password` = pbkdf2(password_as_secret, salt_as_salt)
- `secret` = pbkdf2(password_as_secret, username_as_salt)
- `master` = aesgcm_encrypt(password, masterKey)
- password is used just to decrypt master
- master and secret can be updated when username or password changes
- decrypted `master` (`state.masterKey` in app) never changes
- each encryption result is a triplet of version(1) + base64(salt) + base64(encrypted_data)
- pin is pbkdf2(pin_as_secret, username_as_salt) user to encrypt user's password
- could probably move to secret being derived from user salt too and concatenate salt with fixed prefix and hash again
