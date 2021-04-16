

robot-start:
	-sudo rm /etc/systemd/system/viamrobot.service
	sudo ln -s `pwd`/viamrobot.service /etc/systemd/system/
	sudo systemctl enable viamrobot
	sudo systemctl start viamrobot

ln-server:
	sudo ln -s `pwd`/../bin/Linux-aarch64/server /usr/local/bin/viamserver


