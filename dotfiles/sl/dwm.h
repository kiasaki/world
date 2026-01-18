#include <X11/XF86keysym.h>

static const unsigned int borderpx = 1;
static const unsigned int snap = 32;
static const int showbar = 1;
static const int topbar = 0;
static const char *fonts[] = {"Berkeley Mono:size=20:antialias=true:autohint=true"};
static const char dmenufont[] = "Berkeley Mono:size=20:antialias=true:autohint=true";
static const char col_white[] = "#f6f6f6";
static const char col_black[] = "#222222";
static const char col_gray1[] = "#eeeeee";
static const char col_gray2[] = "#bbbbbb";
static const char col_gray3[] = "#444444";
static const char col_gray4[] = "#222222";
static const char col_cyan[] = "#ec4899";
static const char *colors[][3] = {
	[SchemeNorm] = {col_white, col_black, col_black},
	[SchemeSel] = {col_black, col_white, col_black},
};

static const char *tags[] = {"1", "2", "3", "4"};

static const Rule rules[] = {
	/* class      instance    title       tags mask     isfloating   monitor */
  {"Gimp", NULL, NULL, 0, 1, -1},
  {"Spectacle", NULL, NULL, 0, 1, -1},
  //{"Firefox", NULL, NULL, 1 << 4, 0, -1},
  //{"Chromium", NULL, NULL, 1 << 4, 0, -1},
};

/* layout(s) */
static const float mfact = 0.6;
static const int nmaster = 1;
static const int resizehints = 1;
static const int lockfullscreen = 1;

static const Layout layouts[] = {
	{ "[M]",      monocle },
	{ "[]=",      tile },
	{ "><>",      NULL },
};

/* key definitions */
#define MODKEY Mod1Mask
#define TAGKEYS(KEY,TAG) \
	{ MODKEY,                       KEY,      view,           {.ui = 1 << TAG} }, \
	{ MODKEY|ControlMask,           KEY,      toggleview,     {.ui = 1 << TAG} }, \
	{ MODKEY|ShiftMask,             KEY,      tag,            {.ui = 1 << TAG} }, \
	{ MODKEY|ControlMask|ShiftMask, KEY,      toggletag,      {.ui = 1 << TAG} },

#define SHCMD(cmd) { .v = (const char*[]){ "/bin/sh", "-c", cmd, NULL } }

/* commands */
static char dmenumon[2] = "0";
static const char *dmenucmd[] = {"dmenu_run", "-m", dmenumon, "-fn", dmenufont, "-nb", col_gray1, "-nf", col_gray3, "-sb", col_cyan, "-sf", col_gray4, NULL};
static const char *termcmd[]  = {"st", NULL};
static const char *mutecmd[] = {"amixer", "-q", "set", "Master", "toggle", NULL};
static const char *volupcmd[] = {"amixer", "-q", "set", "Master", "5%+", "unmute", NULL};
static const char *voldowncmd[] = {"amixer", "-q", "set", "Master", "5%-", "unmute", NULL};
static const char *musicprevcmd[] = {"mocp", "--previous", NULL};
static const char *musicplaycmd[] = {"mocp", "--toggle-pause", NULL};
static const char *musicnextcmd[] = {"mocp", "--next", NULL};
// add %wheel ALL=(ALL) NOPASSWD: /usr/bin/xbacklight
static const char *brupcmd[] = {"sudo", "xbacklight", "-inc", "10", NULL};
static const char *brdowncmd[] = {"sudo", "xbacklight", "-dec", "10", NULL};
static const char *intscreen[] = {"intscreen", NULL};
static const char *extscreen[] = {"extscreen", NULL};
static const char *webcmd[] = {"chromium", "--high-dpi-support=1", "--force-device-scale-factor=2.0", NULL};

static const Key keys[] = {
  {MODKEY, XK_p, spawn, {.v = dmenucmd}},
  {MODKEY, XK_w, spawn, {.v = webcmd}},
  {MODKEY, XK_t, spawn, {.v = termcmd}},
  {MODKEY | ShiftMask, XK_Return, spawn, {.v = termcmd}},
  {MODKEY, XK_b, togglebar, {0}},
  {MODKEY, XK_j, focusstack, {.i = +1}},
  {MODKEY, XK_k, focusstack, {.i = -1}},
  {MODKEY, XK_i, incnmaster, {.i = +1}},
  {MODKEY, XK_d, incnmaster, {.i = -1}},
  {MODKEY, XK_h, setmfact, {.f = -0.05}},
  {MODKEY, XK_l, setmfact, {.f = +0.05}},
  {MODKEY, XK_Return, zoom, {0}},
  {MODKEY, XK_Tab, view, {0}},
  {MODKEY | ShiftMask, XK_c, killclient, {0}},
  {MODKEY, XK_t, setlayout, {.v = &layouts[0]}},
  {MODKEY, XK_f, setlayout, {.v = &layouts[1]}},
  {MODKEY, XK_m, setlayout, {.v = &layouts[2]}},
  {MODKEY, XK_space, setlayout, {0}},
  {MODKEY | ShiftMask, XK_space, togglefloating, {0}},
  {MODKEY, XK_0, view, {.ui = ~0}},
  {MODKEY | ShiftMask, XK_0, tag, {.ui = ~0}},
  {MODKEY, XK_comma, focusmon, {.i = -1}},
  {MODKEY, XK_period, focusmon, {.i = +1}},
  {MODKEY | ShiftMask, XK_comma, tagmon, {.i = -1}},
  {MODKEY | ShiftMask, XK_period, tagmon, {.i = +1}},
  TAGKEYS(XK_1, 0) TAGKEYS(XK_2, 1) TAGKEYS(XK_3, 2) TAGKEYS(XK_4, 3)
  {MODKEY | ShiftMask, XK_q, quit, {0}},
  {MODKEY | ShiftMask, XK_i, spawn, {.v = intscreen}},
  {MODKEY | ShiftMask, XK_e, spawn, {.v = extscreen}},
  {0, XF86XK_AudioMute, spawn, {.v = mutecmd}},
  {0, XF86XK_AudioLowerVolume, spawn, {.v = voldowncmd}},
  {0, XF86XK_AudioRaiseVolume, spawn, {.v = volupcmd}},
  {0, XF86XK_AudioPrev, spawn, {.v = musicprevcmd}},
  {0, XF86XK_AudioPlay, spawn, {.v = musicplaycmd}},
  {0, XF86XK_AudioNext, spawn, {.v = musicnextcmd}},
  {0, XF86XK_MonBrightnessUp, spawn, {.v = brupcmd}},
  {0, XF86XK_MonBrightnessDown, spawn, {.v = brdowncmd}},
};

/* button definitions */
static const Button buttons[] = {
  {ClkLtSymbol, 0, Button1, setlayout, {0}},
  {ClkLtSymbol, 0, Button3, setlayout, {.v = &layouts[2]}},
  {ClkWinTitle, 0, Button2, zoom, {0}},
  {ClkStatusText, 0, Button2, spawn, {.v = termcmd}},
  {ClkClientWin, MODKEY, Button1, movemouse, {0}},
  {ClkClientWin, MODKEY, Button2, togglefloating, {0}},
  {ClkClientWin, MODKEY, Button3, resizemouse, {0}},
  {ClkTagBar, 0, Button1, view, {0}},
  {ClkTagBar, 0, Button3, toggleview, {0}},
  {ClkTagBar, MODKEY, Button1, tag, {0}},
  {ClkTagBar, MODKEY, Button3, toggletag, {0}},
};

