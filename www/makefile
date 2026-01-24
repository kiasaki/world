run:
	python3 -m http.server

thumbs:
	bash -c 'for img in pictures/*.jpg; do base=$$(basename "$$img"); magick convert "$$img" -resize 300x300 "thumbnails/$$base"; done)'

deploy:
	rsync -rv --del --exclude .git/ --exclude makefile . home:/home/ubuntu/websites/kiasaki.com/
