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
- key: api key, shared secret for access control
- master: encrypted master key stored on user
- datum: EAVT quadruplet representing database data
- chunk: ~1mb array of datums stored together
- checkpoint: `chunk.index` used by app to track sync position
- blob: ~1mb encrypted, content addressed data stored outside of synced db

## s3 storage paths

- `n/<hashed-username>`: user key as string
- `u/<key>`: user data (master, chunk, key (not returned))
- `d/<key>/<chunk>`: chunk of datums
- `b/<key>/<hashed-data>`: blob of data
