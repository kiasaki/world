source ~/.bashrc
xset s noblank
xset s noexpose
setxkbmap -layout us -option ctrl:nocaps &
ksync &
test -f ~/.fehbg && ~/.fehbg & # feg --bg-fill bg.jpg
export K_SCALE=2
kbar&
#dwmstatus 2>&1 >/dev/null &
while xsetroot -name "`date +'%b %d %a %H:%M'` | M`free -h | awk '(NR==2){ print substr($3, 1, length($4)-2) }'` | D`df -h | awk '{ if ($6 == "/") print substr($4, 1, length($4)-1) }'` | V`amixer get Master | awk -F'[][]' 'END{ print substr($2, 1, length($2)-1) }'` | B`cat /sys/class/power_supply/BAT1/capacity` "
do
	sleep 1
done &
#while true; do
	kwm >/dev/null 2>&1 || dwm
#done
