package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unsafe"
)

type Editor struct {
	Mode       int // 0 normal 1 insert 2 visual
	X          int
	Y          int
	OffsetX    int
	OffsetY    int
	RenderedX  int
	Height     int
	Width      int
	Lines      []string
	Name       string
	Message    string
	MarkX      int
	MarkY      int
	VisualX    int
	VisualY    int
	Search     string
	Undos      []*Action
	Redos      []*Action
	LastAction time.Time
}

type Action struct {
	Kind   int // 0 edit 1 lineadd 2 linedel 3 group
	X, Y   int
	Insert string
	Delete string
}

func main() {
	e := &Editor{}
	e.Lines = []string{""}
	termRaw()
	defer func() {
		termReset()
		io.WriteString(os.Stdout, "\x1b[2J")
		io.WriteString(os.Stdout, "\x1b[H")
		if err := recover(); err != nil {
			e.Save()
			panic(err)
		}
	}()
	e.UpdateSize()
	if len(os.Args) >= 2 {
		e.Name = os.Args[1]
		e.Open()
	}
	for {
		e.Render()
		e.Input()
	}
}

func (e *Editor) UpdateSize() {
	height, width, err := termWindowSize()
	die(err)
	e.Height = height
	e.Width = width
}

func (e *Editor) Render() {
	e.OffsetX = min(e.X, max(e.X-e.Width+1, e.OffsetX))
	e.OffsetY = min(e.Y, max(e.Y-(e.Height-2)+1, e.OffsetY))

	inVisual := false
	x1, y1, x2, y2 := e.VisualRange()

	e.RenderedX = 0
	ab := bytes.NewBufferString("\x1b[25l")
	ab.WriteString("\x1b[H")
	for sy := 0; sy < e.Height-2; sy++ {
		y := sy + e.OffsetY
		if inVisual && e.Mode >= 2 && y > y2 {
			ab.WriteString("\x1b[m")
		}
		if y >= len(e.Lines) {
			ab.WriteString("~")
		} else {
			var i int
			for j, c := range e.Lines[y][min(len(e.Lines[y]), e.OffsetX):] {
				if y == e.Y && j == e.X-e.OffsetX {
					e.RenderedX = i
				}
				if i >= e.Width {
					break
				}
				if !inVisual && e.Mode >= 2 && y >= y1 && j+e.OffsetX >= x1 {
					inVisual = true
					ab.WriteString("\x1b[97;100m")
				}
				if inVisual && e.Mode >= 2 && y == y2 && j+e.OffsetX > x2 {
					ab.WriteString("\x1b[m")
				}
				if c == '\t' {
					ab.WriteByte(' ')
					i++
					for i%4 != 0 {
						ab.WriteByte(' ')
						i++
					}
				} else if unicode.IsControl(rune(c)) {
					ab.WriteString("\x1b[7m")
					if c < 26 {
						ab.WriteString("@")
					} else {
						ab.WriteString("?")
					}
					ab.WriteString("\x1b[m")
					i++
				} else {
					ab.WriteString(string(c))
					i++
				}
			}
			if y == e.Y && e.X == len(e.Lines[y]) {
				e.RenderedX = i
			}
		}
		ab.WriteString("\x1b[K")
		ab.WriteString("\r\n")
	}

	// status
	ab.WriteString("\x1b[7m")
	name := e.Name
	if name == "" {
		name = "?"
	}
	status := fmt.Sprintf("%.20s", name) // add modified+ft
	statusw := len(status)
	if statusw > e.Width {
		statusw = e.Width
	}
	rstatus := fmt.Sprintf("%d %d/%d", e.X+1, e.Y+1, len(e.Lines))
	rstatusw := len(rstatus)
	ab.WriteString(status[:statusw])
	for statusw < e.Width {
		if e.Width-statusw == rstatusw {
			ab.WriteString(rstatus)
			break
		} else {
			ab.WriteString(" ")
			statusw++
		}
	}
	ab.WriteString("\x1b[m")
	ab.WriteString("\r\n")

	// message
	ab.WriteString("\x1b[K")
	messagew := len(e.Message)
	if messagew > e.Width {
		messagew = e.Width
	}
	ab.WriteString(e.Message[:messagew])

	// write
	ab.WriteString(fmt.Sprintf("\x1b[%d;%dH", (e.Y-e.OffsetY)+1, e.RenderedX+1))
	ab.WriteString("\x1b[?25h")
	_, err := ab.WriteTo(os.Stdout)
	die(err)
}

func (e *Editor) Input() {
	c := termReadKey()
	if e.Mode == 0 || e.Mode >= 2 { // normal or visual
		switch c {
		case ('q' & 0x1f):
			e.Quit()
		case ('s' & 0x1f):
			e.Save()
		case 'h':
			e.Move(0, -1)
		case 'j':
			e.Move(1, 0)
		case 'k':
			e.Move(-1, 0)
		case 'l':
			e.Move(0, 1)
		case 'w':
			e.MoveSkip(1, true)
			e.MoveSkip(1, false)
		case 'e':
			e.MoveSkip(1, false)
			e.MoveSkip(1, true)
			e.Move(0, -1)
		case 'b':
			e.MoveSkip(-1, false)
			e.MoveSkip(-1, true)
			e.Move(0, 1)
		case HOME_KEY, '0':
			e.X = 0
		case END_KEY, '$':
			if e.Y < len(e.Lines) {
				e.X = len(e.Lines[e.Y])
			}
		case PAGE_UP, ('u' & 0x1f):
			e.Move(-1*(e.Height/2), 0)
		case PAGE_DOWN, ('d' & 0x1f):
			e.Move(e.Height/2, 0)
		case 'g':
			e.Y = 0
			e.X = 0
		case 'G':
			e.Y = len(e.Lines) - 1
			e.X = 0
		case 'z':
			e.OffsetY = max(0, e.Y-((e.Height-3)/2))
		case ':':
			e.Command(e.Promt(":", nil))
		case '\x1b':
			e.Mode = 0
			e.Message = ""
		case 'v':
			e.Mode = 2
			e.Message = "-- VISUAL --"
			e.VisualX = e.X
			e.VisualY = e.Y
		case 'V':
			e.Mode = 3
			e.Message = "-- VISUAL LINE --"
			e.VisualX = 0
			e.VisualY = e.Y
		case 'i':
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'I':
			e.X = 0
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'a':
			e.Move(0, 1)
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'A':
			if e.Y < len(e.Lines) {
				e.X = len(e.Lines[e.Y])
			}
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'o':
			if e.Y < len(e.Lines) {
				e.X = len(e.Lines[e.Y])
			}
			e.InsertNewline()
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'O':
			e.X = 0
			e.InsertNewline()
			e.Move(0, -1)
			e.Mode = 1
			e.Message = "-- INSERT --"
		case 'x':
			e.Move(0, 1)
			e.Delete()
		case 'm':
			e.MarkX = e.X
			e.MarkY = e.Y
		case '\'':
			e.Y = max(0, min(len(e.Lines)-1, e.MarkY))
			e.X = max(0, min(len(e.Lines[e.Y]), e.MarkX))
		case 'c':
			if e.Mode >= 2 {
				clipboardWrite(e.Range(e.VisualRange()))
				e.DeleteVisual()
				e.Mode = 1
				e.Message = "-- INSERT --"
			}
		case 'd':
			if e.Mode >= 2 {
				clipboardWrite(e.Range(e.VisualRange()))
				e.DeleteVisual()
				e.Mode = 0
				e.Message = ""
			} else {
				clipboardWrite(e.Lines[e.Y] + "\n")
				e.Edit(2, e.Y, 0, e.Lines[e.Y], "")
				if e.Y >= len(e.Lines) {
					e.Y = len(e.Lines) - 1
				}
				e.X = 0
			}
		case 'y':
			if e.Mode >= 2 {
				clipboardWrite(e.Range(e.VisualRange()))
				e.Mode = 0
				e.Message = ""
			} else {
				clipboardWrite(e.Lines[e.Y])
			}
		case 'p':
			if e.Mode >= 2 {
				s := e.Range(e.VisualRange())
				e.DeleteVisual()
				e.Insert(clipboardRead())
				clipboardWrite(s)
				e.Mode = 0
				e.Message = ""
			} else {
				e.Insert(clipboardRead())
			}
		case 'u':
			e.Undo()
		case 'r':
			e.Redo()
		case '/':
			e.Search = e.Promt("/", nil)
			e.SearchMove(1)
		case 'n':
			e.SearchMove(1)
		case 'N':
			e.SearchMove(-1)
		default:
			e.Message = fmt.Sprintf("%d %s", c, string(rune(c)))
		}
	} else {
		switch c {
		case ('q' & 0x1f):
			e.Quit()
		case '\x1b':
			e.Mode = 0
			e.Message = ""
		case ('l' & 0x1f):
			break
		case '\r':
			e.InsertNewline()
		case '\t':
			e.Insert("    ")
		case ('h' & 0x1f), BACKSPACE, DEL_KEY:
			if c == DEL_KEY {
				e.Move(0, 1)
			}
			e.Delete()
		case ARROW_LEFT:
			e.Move(0, -1)
		case ARROW_DOWN:
			e.Move(1, 0)
		case ARROW_UP:
			e.Move(-1, 0)
		case ARROW_RIGHT:
			e.Move(0, 1)
		default:
			if unicode.IsPrint(rune(byte(c))) {
				e.Insert(string(rune(byte(c))))
			}
		}
	}
}

func (e *Editor) Move(y, x int) {
	e.Y = max(0, min(len(e.Lines)-1, e.Y+y))
	e.X = max(0, min(len(e.Lines[e.Y]), e.X))
	if x < 0 {
		for i := 0; i < 0-x; i++ {
			if e.X != 0 {
				e.X--
			} else if e.Y > 0 {
				e.Y--
				e.X = len(e.Lines[e.Y])
			}
		}
	} else {
		for i := 0; i < x; i++ {
			if e.X < len(e.Lines[e.Y]) {
				e.X++
			} else if e.Y < len(e.Lines)-1 {
				e.Y++
				e.X = 0
			}
		}
	}
}

func (e *Editor) SearchMove(dir int) {
	initialY := -1
	for {
		i := strings.Index(e.Lines[e.Y][e.X:], e.Search)
		if i > 0 {
			e.X += i
			return
		}
		if e.Y == initialY {
			return
		}
		initialY = e.Y
		e.Y += dir
		e.X = 0
		if e.Y == len(e.Lines) {
			e.Y = 0
		}
	}
}

func (e *Editor) MoveSkip(dir int, alpha bool) {
	for {
		e.Move(0, dir)
		if e.X == len(e.Lines[e.Y]) {
			return
		}
		c := rune(e.Lines[e.Y][e.X])
		ca := unicode.IsDigit(c) || unicode.IsLetter(c)
		if alpha && !ca {
			return
		}
		if !alpha && ca {
			return
		}
	}
}

func (e *Editor) Command(cmd string) {
	if cmd == "" {
		return
	}
	parts := strings.Split(cmd, " ")
	if parts[0] == "w" {
		if len(parts) > 1 && parts[1] != "" {
			e.Name = parts[1]
		}
		e.Save()
	} else if parts[0] == "e" {
		if len(parts) > 1 && parts[1] != "" {
			e.Name = parts[1]
		}
		e.Open()
	} else if parts[0] == "wq" {
		e.Save()
		e.Quit()
	} else if parts[0] == "q" {
		e.Quit()
	}
}

func (e *Editor) Promt(prompt string, callback func([]byte, int)) string {
	var buf []byte
	for {
		e.Message = prompt + string(buf)
		e.Render()
		c := termReadKey()
		if c == DEL_KEY || c == ('h'&0x1f) || c == BACKSPACE {
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		} else if c == ('u' & 0x1f) {
			buf = nil
		} else if c == '\x1b' || c == ('c'&0x1f) {
			e.Message = ""
			if callback != nil {
				callback(buf, c)
			}
			return ""
		} else if c == '\r' {
			if len(buf) != 0 {
				e.Message = ""
				if callback != nil {
					callback(buf, c)
				}
				return string(buf)
			}
		} else {
			if unicode.IsPrint(rune(c)) {
				buf = append(buf, byte(c))
			}
		}
		if callback != nil {
			callback(buf, c)
		}
	}
}

func (e *Editor) Quit() {
	termReset()
	io.WriteString(os.Stdout, "\x1b[2J")
	io.WriteString(os.Stdout, "\x1b[H")
	os.Exit(0)
}

func (e *Editor) Open() {
	if _, e := os.Stat(e.Name); e != nil {
		return
	}
	s, err := os.ReadFile(e.Name)
	die(err)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	e.Lines = strings.Split(string(s), "\n")
	e.Move(0, 0)
}

func (e *Editor) Save() {
	if e.Name == "" {
		e.Message = "did not save, no name"
		return
	}
	bs := []byte(strings.Join(e.Lines, "\n") + "\n")
	err := os.WriteFile(e.Name, bs, 0644)
	if err != nil {
		e.Message = fmt.Sprintf("error: %v", err)
	}
	if strings.HasSuffix(e.Name, ".go") {
		e.Message = "running..."
		if err := exec.Command("goimports", "-w", e.Name).Run(); err != nil {
			e.Message = fmt.Sprintf("error: goimports: %v", err)
			return
		}
		e.Open()
	}
	e.Message = fmt.Sprintf("wrote %d bytes", len(bs))
}

func (e *Editor) ActionCanMerge(prev, next *Action) bool {
	if next.Kind != 0 || prev.Kind != 0 {
		return false
	}
	if next.Y != prev.Y {
		return false
	}
	if next.Delete == "" && prev.Delete == "" {
		return next.X == prev.X+len(prev.Insert)
	}
	if next.Insert == "" && prev.Insert == "" {
		return next.X == prev.X-len(next.Delete)
	}
	return false
}

func (e *Editor) ActionDo(a *Action) {
	if a.Kind == 3 {
		return
	} else if a.Kind == 2 {
		a.Delete = e.Lines[a.Y]
		e.Lines = append(e.Lines[:a.Y], e.Lines[a.Y+1:]...)
	} else if a.Kind == 1 {
		e.Lines = append(e.Lines[:a.Y], append([]string{a.Insert}, e.Lines[a.Y:]...)...)
	} else {
		line := e.Lines[a.Y]
		if a.Delete != "" {
			a.Delete = line[a.X : a.X+len(a.Delete)]
			line = line[:a.X] + line[a.X+len(a.Delete):]
		}
		if a.Insert != "" {
			line = line[:a.X] + a.Insert + line[a.X:]
		}
		e.Lines[a.Y] = line
	}
}

func (e *Editor) ActionUndo(a *Action) {
	if a.Kind == 3 {
		return
	} else if a.Kind == 2 {
		e.Lines = append(e.Lines[:a.Y], append([]string{a.Delete}, e.Lines[a.Y:]...)...)
	} else if a.Kind == 1 {
		e.Lines = append(e.Lines[:a.Y], e.Lines[a.Y+1:]...)
	} else {
		line := e.Lines[a.Y]
		if a.Insert != "" {
			a.Insert = line[a.X : a.X+len(a.Insert)]
			line = line[:a.X] + line[a.X+len(a.Insert):]
		}
		if a.Delete != "" {
			line = line[:a.X] + a.Delete + line[a.X:]
		}
		e.Lines[a.Y] = line
	}
}

func (e *Editor) ActionMerge(a *Action) {
	action := e.Undos[len(e.Undos)-1]
	action.Insert = action.Insert + a.Insert
	action.Delete = a.Delete + action.Delete
}

func (e *Editor) Edit(kind, y, x int, delete, insert string) {
	action := &Action{Kind: kind, X: x, Y: y, Delete: delete, Insert: insert}
	if time.Now().After(e.LastAction.Add(time.Second)) {
		e.Undos = append(e.Undos, &Action{Kind: 3})
	}
	e.LastAction = time.Now()
	e.Redos = nil
	e.ActionDo(action)

	if len(e.Undos) > 0 {
		prev := e.Undos[len(e.Undos)-1]
		if e.ActionCanMerge(prev, action) {
			e.ActionMerge(action)
			return
		}
	}
	e.Undos = append(e.Undos, action)
}

func (e *Editor) Undo1() *Action {
	if len(e.Undos) == 0 {
		return nil
	}
	action := e.Undos[len(e.Undos)-1]
	e.Undos = e.Undos[:len(e.Undos)-1]
	e.ActionUndo(action)
	e.Redos = append(e.Redos, action)
	return action
}

func (e *Editor) Undo() {
	action := e.Undo1()
	for action != nil && action.Kind != 3 {
		action = e.Undo1()
		if action != nil && action.Kind != 3 {
			e.Y, e.X = action.Y, action.X
		}
	}
	e.Move(0, 0)
}

func (e *Editor) Redo1() *Action {
	if len(e.Redos) == 0 {
		return nil
	}
	action := e.Redos[len(e.Redos)-1]
	e.Redos = e.Redos[:len(e.Redos)-1]
	e.ActionDo(action)
	e.Undos = append(e.Undos, action)
	return action
}

func (e *Editor) Redo() {
	action := e.Redo1()
	if action != nil && action.Kind == 3 {
		action = e.Redo1()
	}
	for action != nil && action.Kind != 3 {
		action = e.Redo1()
		if action != nil && action.Kind != 3 {
			e.Y, e.X = action.Y, action.X
		}
	}
	e.Move(0, 0)
}

func (e *Editor) InsertNewline() {
	tail := e.Lines[e.Y][e.X:]
	e.Edit(0, e.Y, e.X, tail, "")
	e.Edit(1, e.Y+1, 0, "", tail)
	e.Y += 1
	e.X = 0
}

func (e *Editor) Insert(s string) {
	lines := strings.Split(s, "\n")
	if len(lines) == 1 {
		e.Edit(0, e.Y, e.X, "", s)
		e.X += len(s)
		return
	}
	// first line
	e.Edit(0, e.Y, e.X, "", lines[0])
	e.X += len(lines[0])
	for i := 1; i < len(lines); i++ {
		e.InsertNewline()
		e.Edit(0, e.Y, e.X, "", lines[i])
		e.X += len(lines[i])
	}
}

func (e *Editor) Range(x1, y1, x2, y2 int) string {
	if y1 == y2 {
		end := x2 + 1
		if end > len(e.Lines[y1]) {
			end = len(e.Lines[y1])
		}
		return e.Lines[y1][x1:end]
	}
	s := e.Lines[y1][x1:]
	for i := y1 + 1; i < y2; i++ {
		s += "\n" + e.Lines[i]
	}
	end := x2 + 1
	if end > len(e.Lines[y2]) {
		end = len(e.Lines[y2])
	}
	s += "\n" + e.Lines[y2][:end]
	return s
}

func (e *Editor) VisualRange() (int, int, int, int) {
	x1, y1 := e.VisualX, e.VisualY
	x2, y2 := e.X, e.Y
	if e.VisualY*10000+e.VisualX > e.Y*10000+e.X {
		x1, y1 = e.X, e.Y
		x2, y2 = e.VisualX, e.VisualY
	}
	if e.Mode == 3 {
		x1 = 0
		x2 = len(e.Lines[y2])
	}
	return x1, y1, x2, y2
}

func (e *Editor) DeleteVisual() {
	x1, y1, x2, y2 := e.VisualRange()
	if y1 == y2 && x1 == x2 {
		return
	}
	s := e.Range(x1, y1, x2, y2)
	e.Edit(0, y1, x1, s, "")
	e.X, e.Y = x1, y1
	e.Move(0, 0)
}

func (e *Editor) Delete() {
	if e.X == 0 {
		if e.Y > 0 {
			line := e.Lines[e.Y]
			e.X = len(e.Lines[e.Y-1])
			e.Edit(2, e.Y, 0, line, "")
			e.Edit(0, e.Y-1, len(e.Lines[e.Y-1]), "", line)
			e.Y -= 1
		}
		return
	}
	e.Edit(0, e.Y, e.X-1, "?", "")
	e.X -= 1
}

type Termios syscall.Termios

var originalTermios *Termios

func termRaw() {
	originalTermios = termGetAttr(os.Stdin.Fd())
	var raw Termios
	raw = *originalTermios
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN+1] = 0
	raw.Cc[syscall.VTIME+1] = 1
	if e := termSetAttr(os.Stdin.Fd(), &raw); e != nil {
		die(fmt.Errorf("problem enabling raw mode: %s\n", e))
	}
}

func termReset() {
	if e := termSetAttr(os.Stdin.Fd(), originalTermios); e != nil {
		die(fmt.Errorf("problem disabling raw mode: %s\n", e))
	}
}

func termGetAttr(fd uintptr) *Termios {
	getTermios := uintptr(0x5401)
	if runtime.GOOS == "darwin" {
		getTermios = 0x40487413
	}
	var termios = &Termios{}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, getTermios, uintptr(unsafe.Pointer(termios))); err != 0 {
		die(fmt.Errorf("problem getting terminal attributes: %s\n", err))
	}
	return termios
}

func termSetAttr(fd uintptr, termios *Termios) error {
	setTermios := uintptr(0x5402) // +1
	if runtime.GOOS == "darwin" {
		setTermios = 0x80487414
	}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, setTermios, uintptr(unsafe.Pointer(termios))); err != 0 {
		return err
	}
	return nil
}

func termWindowSize() (int, int, error) {
	var w struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&w)),
	)
	if err != 0 { // type syscall.Errno
		io.WriteString(os.Stdout, "\x1b[999C\x1b[999B")
		return termCursorPosition()
	} else {
		return int(w.Row), int(w.Col), nil
	}
}

func termCursorPosition() (int, int, error) {
	io.WriteString(os.Stdout, "\x1b[6n")
	var buffer [1]byte
	var buf []byte
	var cc int
	for cc, _ = os.Stdin.Read(buffer[:]); cc == 1; cc, _ = os.Stdin.Read(buffer[:]) {
		if buffer[0] == 'R' {
			break
		}
		buf = append(buf, buffer[0])
	}
	if string(buf[0:2]) != "\x1b[" {
		return 0, 0, errors.New("failed to read rows;cols from tty")
	}
	var rows, cols int
	if n, e := fmt.Sscanf(string(buf[2:]), "%d;%d", rows, cols); n != 2 || e != nil {
		if e != nil {
			return 0, 0, fmt.Errorf("termCursorPosition: fmt.Sscanf failed: %s", e)
		}
		if n != 2 {
			return 0, 0, fmt.Errorf("termCursorPosition: got %d items, wanted 2", n)
		}
		return rows, cols, nil
	}
	return 0, 0, fmt.Errorf("termCursorPosition: failed")
}

const (
	BACKSPACE   = 127
	ARROW_LEFT  = 1000 + iota
	ARROW_RIGHT = 1000 + iota
	ARROW_UP    = 1000 + iota
	ARROW_DOWN  = 1000 + iota
	DEL_KEY     = 1000 + iota
	HOME_KEY    = 1000 + iota
	END_KEY     = 1000 + iota
	PAGE_UP     = 1000 + iota
	PAGE_DOWN   = 1000 + iota
)

func termReadKey() int {
	var buffer [1]byte
	var cc int
	var err error
	for cc, err = os.Stdin.Read(buffer[:]); cc != 1; cc, err = os.Stdin.Read(buffer[:]) {
	}
	if err != nil {
		die(err)
	}
	if buffer[0] == '\x1b' {
		var seq [2]byte
		if cc, _ = os.Stdin.Read(seq[:]); cc != 2 {
			return '\x1b'
		}

		if seq[0] == '[' {
			if seq[1] >= '0' && seq[1] <= '9' {
				if cc, err = os.Stdin.Read(buffer[:]); cc != 1 {
					return '\x1b'
				}
				if buffer[0] == '~' {
					switch seq[1] {
					case '1':
						return HOME_KEY
					case '3':
						return DEL_KEY
					case '4':
						return END_KEY
					case '5':
						return PAGE_UP
					case '6':
						return PAGE_DOWN
					case '7':
						return HOME_KEY
					case '8':
						return END_KEY
					}
				}
			} else {
				switch seq[1] {
				case 'A':
					return ARROW_UP
				case 'B':
					return ARROW_DOWN
				case 'C':
					return ARROW_RIGHT
				case 'D':
					return ARROW_LEFT
				case 'H':
					return HOME_KEY
				case 'F':
					return END_KEY
				}
			}
		} else if seq[0] == '0' {
			switch seq[1] {
			case 'H':
				return HOME_KEY
			case 'F':
				return END_KEY
			}
		}

		return '\x1b'
	}
	return int(buffer[0])
}

func die(err error) {
	if err == nil {
		return
	}
	termReset()
	io.WriteString(os.Stdout, "\x1b[2J")
	io.WriteString(os.Stdout, "\x1b[H")
	log.Fatal(err)
}

func clipboardRead() string {
	args := []string{"xclip", "-out", "-selection", "clipboard"}
	if runtime.GOOS == "darwin" {
		args = []string{"pbpaste"}
	}
	cmd := exec.Command(args[0], args[1:]...)
	bs, err := cmd.Output()
	die(err)
	return string(bs)
}

func clipboardWrite(s string) {
	args := []string{"xclip", "-in", "-selection", "clipboard"}
	if runtime.GOOS == "darwin" {
		args = []string{"pbcopy"}
	}
	cmd := exec.Command(args[0], args[1:]...)
	in, err := cmd.StdinPipe()
	die(err)
	die(cmd.Start())
	in.Write([]byte(s))
	die(in.Close())
	die(cmd.Wait())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
