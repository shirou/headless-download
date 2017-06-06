CHROME=/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome

start:
	${CHROME} --headless --remote-debugging-port=9222 --disable-gpu
