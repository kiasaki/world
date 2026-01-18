static int topbar = 0;
static const char *fonts[] = {"Berkeley Mono:size=20:antialias=true:autohint=true"};
static const char *prompt = NULL;
static unsigned int lines = 0;
static const char worddelimiters[] = " ";
static const char *colors[SchemeLast][2] = {
  [SchemeNorm] = {"#222222", "#f6f6f6"},
  [SchemeSel] = {"#f6f6f6", "#222222"},
  [SchemeOut] = {"#000000", "#00ffff"},
};
