robot-start:
	-sudo rm /etc/systemd/system/viamrobot.service
	sudo ln -s `pwd`/viamrobot.service /etc/systemd/system/
	sudo systemctl start viamrobot
	sudo systemctl enable viamrobot
