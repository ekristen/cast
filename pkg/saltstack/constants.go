package saltstack

const (
	Version string = "3006.9" // renovate: datasource=github-releases depName=saltstack/salt

	OneDirURL     string = `https://packages.broadcom.com/artifactory/saltproject-generic/onedir/{{ .Version }}/salt-{{ .Version }}-onedir-linux-{{ .OS.Architecture }}.tar.xz`
	OneDirHashURL string = `https://packages.broadcom.com/artifactory/saltproject-generic/onedir/{{ .Version }}/salt-{{ .Version }}-onedir-linux-{{ .OS.Architecture }}.tar.xz.sha256`
	RepoKeyURL    string = "https://packages.broadcom.com/artifactory/api/security/keypair/SaltProjectKey/public"
	RepoKeyFile   string = "/etc/apt/keyrings/salt-archive-keyring-2023.pgp"
	APTRepo       string = "deb [signed-by=/etc/apt/keyrings/salt-archive-keyring-2023.pgp arch={{ .OS.Architecture }}] https://packages.broadcom.com/artifactory/saltproject-deb/ stable main"
)

const PublicKey string = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFOpvpgBCADkP656H41i8fpplEEB8IeLhugyC2rTEwwSclb8tQNYtUiGdna9
m38kb0OS2DDrEdtdQb2hWCnswxaAkUunb2qq18vd3dBvlnI+C4/xu5ksZZkRj+fW
tArNR18V+2jkwcG26m8AxIrT+m4M6/bgnSfHTBtT5adNfVcTHqiT1JtCbQcXmwVw
WbqS6v/LhcsBE//SHne4uBCK/GHxZHhQ5jz5h+3vWeV4gvxS3Xu6v1IlIpLDwUts
kT1DumfynYnnZmWTGc6SYyIFXTPJLtnoWDb9OBdWgZxXfHEcBsKGha+bXO+m2tHA
gNneN9i5f8oNxo5njrL8jkCckOpNpng18BKXABEBAAG0MlNhbHRTdGFjayBQYWNr
YWdpbmcgVGVhbSA8cGFja2FnaW5nQHNhbHRzdGFjay5jb20+iQFPBBMBAgAiBQJT
qb6YAhsDBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAhCRAOCKFJ3le/vhYhBHVK
GnrnMfFl1ebUvQ4IoUneV7++GSoH/RBbMQtl/h3ztYCnOiUsb7Omwkq23+55cvVb
UoDskguFcHW7K9U5G3y4D0WOYTTzejXITrrALJrtXyGM2faWQtdug5R3VRMSuVGD
UZhsi375U+xxPEfHCmMMbEMHX9+JfaicrlISm6Pgh4g8lUT+4s0DBnJ9fwMCstUn
mHyWSfCIeVAsaWc6726UQB6PAAq30JTihhOzNgzjGtu2MH99H6Y+diHZ8GhA7h38
1CJ6MgTJ30gQQx3/TcAlAG4y7Hra2MD83C9D16o2AvD02KwU0dFa0/iOEA6yyG/V
Zb7M/O7ejOg76UseLFQjPt+pGTfyrxD2g1HfUq+uRI2sVsDcPz25AQ0EU6m+mAEI
AMnv2IfXQB8B+lMERDBrq/NQjcSW2pv1pnTFtlXxX/yme5Tuuoztn6cmCV2JsWTi
AUbZscIbULloQf8WZw4y9MoOElIVBTG6GDG20wCtebZzbY5LLWGmydUbrwbfY1qD
9AUQn77eE2dvIZmkec+JiB6PcWt6tAnO/jnrAizFEMy8nU1lAHe3CgKrBqkLoUkT
aG6RJ1YHnIJilaVklcErOVjP+DAP8WsYvnvxasuErDdS+cmaWmpBoMrCZXazsyyh
miALrCQcT3aQY8bYaahEPcAuOYK83UQ+wixAafClxexGlhUKhbt7adbjBR/siQVn
3jI4KZSrpat5Yjg2CBFwULkAEQEAAYkBNgQYAQIACQUCU6m+mAIbDAAhCRAOCKFJ
3le/vhYhBHVKGnrnMfFl1ebUvQ4IoUneV7++PzYIANPJBHQZIllsCWVGRMu3Clln
II50bjh3eKz3k/r0GzlhruseG8blX/Wk4mJH+2Y1RdpT5/exFzhBhVj2XmWx+L3U
RR/YzLT/q6y7evt3PY9DPEiCWMAS9fgjaRAvwbh4/0Wv3JpxuWTYWHm5u+oeX0/Y
j1vXxcMN+hcxZGuFT5bwOKAe3mwbarhYN7HUDVPzJk1VeIzxVl8YeKEQw0fvDdW3
D52wddW+Wq8vfRf5qq+0YEPmBRdLikRN0imu2vunCLhxGe2Wvs4T2kJcwDmZcX7g
ZknLpvLkE6xBeflJuc5MAuxF03R9XKP/v1f36TrcSIOFzuJHLoGXn1ZuREX1UjA=
=hp2Z
-----END PGP PUBLIC KEY BLOCK-----
`
