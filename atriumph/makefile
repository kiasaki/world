run:
	python3 -m http.server

deploy:
	rsync -rv --del --exclude .git/ --exclude makefile . home:/home/ubuntu/websites/atriumph.com/
