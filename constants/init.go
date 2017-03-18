package constants

const (
	ISAAX_HOME_DIR    = "/tmp/isaax-agent/"
	ISAAX_DAEMON_DIR  = "/opt/"
	ISAAX_CONF_DIR    = "/etc/"
	ISAAX_APP_DIR     = "/var/isaax/project/"
	UNTAR             = "tar xvf"
	NUMBER_OF_RETRIES = 5

	DARWIN_DISKUTIL     = "diskutil"
	DARWIN_UNMOUNT_DISK = "unmountDisk"

	LINUX_DD    = "dd"
	LINUX_MOUNT = "mount"

	GENERAL_MOUNT_FOLDER = "/tmp/isaax-sd/"
	GENERAL_EJECT        = "eject"
	GENERAL_UNMOUNT      = "umount"

	LOCALE_F       string = "locale.conf"
	KEYBOAD_F      string = "vconsole.conf"
	WPA_SUPPLICANT string = "wpa_supplicant.conf"
	INTERFACES_F   string = "interfaces"
	RESOLV_CONF    string = "resolv.conf"

	LANG        = "LANGUAGE=%s\n"
	LOCALE      = "LC_ALL=%s\n"
	LOCALE_LANG = "LANG=%s\n"

	KEYMAP = "KEYMAP=%s\n"

	WPA_CONF = "ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev \n" +
		"update_config=1 \n\n" +
		"network={ \n" +
		"\tssid=\"%s\" \n" +
		"\tpsk=\"%s\" \n" +
		"}\n"

	INTERFACE_WLAN string = "source-directory /etc/network/interfaces.d\n" +
		"\n" +
		"auto lo\n" +
		"iface lo inet loopback\n" +
		"\n" +
		"iface eth0 inet manual\n" +
		"\n" +
		"allow-hotplug wlan0\n" +
		"iface wlan0 inet static\n" +
		"address %s\n" +
		"netmask %s\n" +
		"network %s\n" +
		"gateway %s\n" +
		"dns-nameservers %s\n" +
		"wpa-conf /etc/wpa_supplicant/wpa_supplicant.conf\n"

	INTERFACE_ETH string = "source-directory /etc/network/interfaces.d\n" +
		"\n" +
		"auto lo\n" +
		"iface lo inet loopback\n" +
		"\n" +
		"auto eth0\n" +
		"iface eth0 inet static\n" +
		"address %s\n" +
		"netmask %s\n" +
		"network %s\n" +
		"gateway %s\n" +
		"dns-nameservers %s 8.8.8.8\n" +
		"\n" +
		"iface default inet dhcp\n"

	RESOLV string = "nameserver %s\n"
)
