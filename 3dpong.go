package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/qeedquan/go-media/image/ttf"
	"github.com/qeedquan/go-media/math/ga"
	"github.com/qeedquan/go-media/math/ga/vec2"
	"github.com/qeedquan/go-media/math/ga/vec3"
	"github.com/qeedquan/go-media/sdl"
	"github.com/qeedquan/go-media/sdl/sdlimage/sdlcolor"
	"github.com/qeedquan/go-media/sdl/sdlmixer"
	"github.com/qeedquan/go-media/sdl/sdlttf"
)

const (
	HANDBALL = iota
	ONE_PLAYER
	TWO_PLAYERS
)

var (
	game *Game
)

func main() {
	runtime.LockOSThread()
	rand.Seed(time.Now().UnixNano())
	game = NewGame()
	parseFlags()
	initSDL()
	game.Play()
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: 3dpong [options]")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Controls:")
	fmt.Fprintln(os.Stderr, "    [V] - Change View")
	fmt.Fprintln(os.Stderr, "    [3] Toggle 3D glasses mode")
	fmt.Fprintln(os.Stderr, "    [C] Toggle \"noclick\" mode")
	fmt.Fprintln(os.Stderr, "    [R] Reset the game")
	fmt.Fprintln(os.Stderr, "    [Q] Quit")

	os.Exit(2)
}

func parseFlags() {
	game.Assets = filepath.Join(sdl.GetBasePath(), "assets")
	flag.StringVar(&game.Assets, "assets", game.Assets, "assets directory")
	flag.Float64Var(&game.Net, "net", game.Net, "size of net")
	flag.Float64Var(&game.Gravity, "gravity", game.Gravity, "gravity")
	flag.BoolVar(&game.NoClick[0], "noclick1", game.NoClick[0], "no click for player 1")
	flag.BoolVar(&game.NoClick[1], "noclick2", game.NoClick[1], "no click for player 2")
	flag.BoolVar(&game.Fullscreen, "fullscreen", game.Fullscreen, "fullscreen mode")
	flag.BoolVar(&game.Sound, "sound", game.Sound, "sound")
	flag.IntVar(&game.Mode, "mode", game.Mode, "game mode (0: handball, 1: one player, 2: two player)")

	flag.Usage = usage
	flag.Parse()

	game.Gravity = math.Abs(game.Gravity)
	if game.Gravity < game.MinHandballGravity {
		game.Gravity = game.MinHandballGravity
	}

	game.Bound[0] = image.Rect(0, 0, game.Width, game.Height)
	if game.Mode == TWO_PLAYERS {
		game.Bound[1] = image.Rect(game.Width*3/2, 0, game.Width*5/2, game.Height)
		game.Width = game.Bound[1].Max.X
	}
}

func initSDL() {
	err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_TIMER)
	ck(err)

	err = sdl.InitSubSystem(sdl.INIT_AUDIO)
	ek(err)

	err = sdlmixer.OpenAudio(44100, sdl.AUDIO_S16, 2, 8192)
	ek(err)

	sdlmixer.AllocateChannels(128)

	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "best")

	width, height := game.Width, game.Height
	wflag := sdl.WINDOW_RESIZABLE
	if game.Fullscreen {
		wflag |= sdl.WINDOW_FULLSCREEN_DESKTOP
	}
	window, renderer, err := sdl.CreateWindowAndRenderer(width, height, wflag)
	ck(err)

	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, width, height)
	ck(err)

	err = sdlttf.Init()
	ck(err)

	font, err := sdlttf.OpenFontMem(ttf.VGA437["default"], 16)
	ck(err)

	surface, err := sdl.CreateRGBSurfaceWithFormat(sdl.SWSURFACE, width, height, 32, sdl.PIXELFORMAT_ABGR8888)
	ck(err)

	game.LoadSound("hit")
	game.LoadSound("score")
	game.LoadSound("wall")

	window.SetTitle("3D Pong")
	renderer.SetLogicalSize(width, height)

	game.Window = window
	game.Renderer = renderer
	game.Texture = texture
	game.Surface = surface
	game.Font = font
	game.FontHeight = font.Height()
}

func ek(err error) {
	if err != nil {
		sdl.LogError(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
	}
}

func ck(err error) {
	if err != nil {
		sdl.LogCritical(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "Error", err.Error(), nil)
		os.Exit(1)
	}
}

type Game struct {
	Window   *sdl.Window
	Renderer *sdl.Renderer
	Texture  *sdl.Texture
	Surface  *sdl.Surface
	RedBlue  [2][2]color.RGBA
	Colors   [2][6]color.RGBA
	Ticker   *time.Ticker
	Assets   string
	Sfx      map[string]*sdlmixer.Chunk

	Mode       int
	Bound      [2]image.Rectangle
	Width      int
	Height     int
	Fullscreen bool
	Sound      bool
	Pause      bool

	Font       *sdlttf.Font
	FontHeight int

	Net       float64
	NetHeight float64

	Arena       ga.Vec3d
	PaddleSize  ga.Vec2d
	GlassOffset float64

	Aspect             float64
	Distance           float64
	Gravity            float64
	MinHandballGravity float64
	ComputerSpeed      float64
	ShimmerTime        int
	Spin               int

	Glasses        [2]int
	Player         [2]ga.Vec2d
	Shimmering     [2]int
	OldButton      [2]int
	OldPos         [2]ga.Vec2d
	View           [2]int
	Score          [2]int
	NoClick        [2]bool
	Queue          [2][]ga.Vec2d
	QueuePos       [2]int
	CrapPos        ga.Vec3d
	CrapVel        ga.Vec3d
	BallInPlay     bool
	BallWaitingFor int
	HighScore      int
	FinalScore     int
	Toggle         bool
	GotHighScore   bool

	AngleDivide float64
	Angle       [2]ga.Vec2d
	CosAngle    [2]ga.Vec2d
	SinAngle    [2]ga.Vec2d

	Quit bool

	BallPos          ga.Vec3d
	BallVel          ga.Vec3d
	BallSpeed        float64
	BallSize         float64
	InitialBallSpeed float64

	Debris      []Debris
	DebrisCount int
	DebrisTime  int
	DebrisMin   int
	DebrisMax   int
	DebrisSpeed int
}

type Debris struct {
	Exist bool
	Time  int
	Pos   ga.Vec3d
	Vel   ga.Vec3d
}

func NewGame() *Game {
	const (
		X_WIDTH  = 100
		Y_HEIGHT = 100
		Z_DEPTH  = 150

		PADDLE_WIDTH  = 25
		PADDLE_HEIGHT = 25

		BALL_SPEED = 2

		GLASS_OFFSET = 10
		DISTANCE     = Z_DEPTH + 100
		ASPECT       = 200

		DEBRIS_TIME  = 50
		DEBRIS_MIN   = 5
		DEBRIS_MAX   = 10
		DEBRIS_SPEED = 2
		NUM_DEBRIS   = 50

		MIN_HANDBALL_GRAVITY = 0.25
		COMPUTER_SPEED       = 5
		BALL_SIZE            = 15
		ANGLE_DIVIDE         = 3

		SHIMMER_TIME = 5

		QUEUE_SIZE = 5
	)

	c := &Game{
		Width:  580,
		Height: 580,
		Colors: [2][6]color.RGBA{
			{
				// red
				{255, 0, 0, 255},
				// blue
				{0, 0, 255, 255},
				// green
				{0, 255, 0, 255},
				// darkred
				{139, 0, 0, 255},
				// darkblue
				{0, 0, 139, 255},
				// darkgreen
				{0, 139, 0, 255},
			},
			{
				// red
				{255, 0, 0, 255},
				// blue
				{0, 0, 255, 255},
				// green
				{0, 255, 0, 255},
				// darkred
				{139, 0, 0, 255},
				// darkblue
				{0, 0, 139, 255},
				// darkgreen
				{0, 139, 0, 255},
			},
		},
		Arena:              ga.Vec3d{X_WIDTH, Y_HEIGHT, Z_DEPTH},
		PaddleSize:         ga.Vec2d{PADDLE_WIDTH, PADDLE_HEIGHT},
		InitialBallSpeed:   BALL_SPEED,
		ComputerSpeed:      COMPUTER_SPEED,
		GlassOffset:        GLASS_OFFSET,
		Aspect:             ASPECT,
		MinHandballGravity: MIN_HANDBALL_GRAVITY,
		Debris:             make([]Debris, NUM_DEBRIS),
		Distance:           DISTANCE,
		DebrisTime:         DEBRIS_TIME,
		DebrisMin:          DEBRIS_MIN,
		DebrisMax:          DEBRIS_MAX,
		DebrisSpeed:        DEBRIS_SPEED,
		BallSize:           BALL_SIZE,
		ShimmerTime:        SHIMMER_TIME,
		AngleDivide:        ANGLE_DIVIDE,
		Sound:              true,
		Sfx:                make(map[string]*sdlmixer.Chunk),
		Mode:               HANDBALL,
		RedBlue: [2][2]color.RGBA{
			{
				{0, 0, 255, 255},
				{255, 0, 0, 255},
			},
			{
				{255, 0, 0, 255},
				{0, 0, 255, 255},
			},
		},
		Ticker: time.NewTicker(80 * time.Millisecond),
	}
	for i := range c.Queue {
		c.Queue[i] = make([]ga.Vec2d, QUEUE_SIZE)
		c.QueuePos[i] = 0
	}
	return c
}

func (c *Game) Play() {
	c.reset()
	for !c.Quit {
		plns := 1
		if c.Mode == TWO_PLAYERS {
			plns = 2
		}
		for {
			ev := sdl.PollEvent()
			if ev == nil {
				break
			}
			for pln := 0; pln < plns; pln++ {
				if c.event(pln, ev) {
					break
				}
			}
		}

		select {
		case <-c.Ticker.C:
			c.update()
		}
		c.draw()
	}
}

func (c *Game) LoadSound(name string) {
	path := filepath.Join(c.Assets, name+".ogg")
	chunk, err := sdlmixer.LoadWAV(path)
	ek(err)
	c.Sfx[name] = chunk
}

func (c *Game) reset() {
	for i := range c.Debris {
		c.Debris[i] = Debris{}
	}
	c.DebrisCount = 0

	for i := 0; i < 2; i++ {
		c.OldButton[i] = -1
		c.Player[i] = ga.Vec2d{}
		c.Shimmering[i] = 0
		c.Glasses[i] = 0
		c.View[i] = 0
		c.Score[i] = 0
		c.Angle[i] = ga.Vec2d{5, 5}
		c.recalculateTrig(i)

		for j := range c.Queue {
			c.Queue[i][j] = ga.Vec2d{}
		}
	}

	c.HighScore = 0
	c.FinalScore = -1
	c.BallPos = ga.Vec3d{}
	c.BallVel = ga.Vec3d{}
	c.BallSpeed = 0
	c.BallInPlay = false
	c.Pause = false
}

func (c *Game) recalculateTrig(i int) {
	c.SinAngle[i].X, c.CosAngle[i].X = math.Sincos(c.Angle[i].X * math.Pi / 180)
	c.SinAngle[i].Y, c.CosAngle[i].Y = math.Sincos(c.Angle[i].Y * math.Pi / 180)
}

func (c *Game) xmouse() int {
	mx, _, _ := sdl.GetMouseState()
	ow, _, _ := c.Renderer.OutputSize()
	v := c.Renderer.Viewport()
	return int(ga.LinearRemap(float64(mx), float64(v.X), float64(ow)-float64(v.X), 0, float64(c.Width)))
}

func (c *Game) event(pln int, ev interface{}) bool {
	switch ev := ev.(type) {
	case sdl.QuitEvent:
		c.Quit = true
	case sdl.KeyUpEvent:
		switch ev.Sym {
		case sdl.K_ESCAPE, sdl.K_q:
			c.Quit = true
		case sdl.K_SPACE, sdl.K_RETURN:
			c.Pause = !c.Pause
		}
	}

	if c.Quit || c.Pause {
		return true
	}

	mx := c.xmouse()
	if !(c.Bound[pln].Min.X <= mx && mx <= c.Bound[pln].Max.X) {
		return false
	}

	switch ev := ev.(type) {
	case sdl.KeyDownEvent:
		switch ev.Sym {
		case sdl.K_3:
			c.Glasses[pln] = 1 - c.Glasses[pln]
		case sdl.K_v:
			c.View[pln] = (c.View[pln] + 1) % 6
		case sdl.K_c:
			c.NoClick[pln] = !c.NoClick[pln]
		case sdl.K_r:
			c.reset()
		}
	case sdl.MouseButtonDownEvent:
		// They clicked!  The beginning of a drag!
		c.OldButton[pln] = int(ev.Button)
		c.OldPos[pln] = ga.Vec2d{float64(ev.X), float64(ev.Y)}

		// If the ball wasn't in play, this person launched it
		if !c.BallInPlay && c.BallWaitingFor == pln && ev.Button == sdl.BUTTON_RIGHT {
			c.putBallInPlay(pln)
		}
	case sdl.MouseButtonUpEvent:
		c.OldButton[pln] = -1
	case sdl.MouseMotionEvent:
		if c.OldButton[pln] == sdl.BUTTON_LEFT || c.NoClick[pln] {
			// Add their moves to their queue
			c.Queue[pln][c.QueuePos[pln]] = ga.Vec2d{float64(ev.Xrel), float64(ev.Yrel)}
			c.QueuePos[pln] = (c.QueuePos[pln] + 1) % len(c.QueuePos)

			// Move Paddle
			c.Player[pln].X += float64(ev.Xrel)
			c.Player[pln].Y += float64(ev.Yrel)

			c.Player[pln].X = ga.Clamp(c.Player[pln].X, -c.Arena.X+c.PaddleSize.X, c.Arena.X-c.PaddleSize.X)
			c.Player[pln].Y = ga.Clamp(c.Player[pln].Y, -c.Arena.Y+c.PaddleSize.Y, c.Arena.Y-c.PaddleSize.Y)

			c.OldPos[pln] = ga.Vec2d{float64(ev.X), float64(ev.Y)}
		} else if c.OldButton[pln] == sdl.BUTTON_MIDDLE {
			c.Angle[pln] = vec2.Add(c.Angle[pln], ga.Vec2d{float64(ev.Xrel), float64(ev.Yrel)})
			c.Angle[pln].X = ga.Wrap(c.Angle[pln].X, 0, 360)
			c.Angle[pln].Y = ga.Wrap(c.Angle[pln].Y, 0, 360)

			c.recalculateTrig(pln)
			c.OldPos[pln] = ga.Vec2d{float64(ev.X), float64(ev.Y)}
		}
	}
	return true
}

func (c *Game) update() {
	if c.Pause {
		return
	}
	c.moveComputer()
	c.moveDebris()
	c.moveCrap()
	c.moveBall()
}

func (c *Game) moveComputer() {
	if c.Mode == ONE_PLAYER {
		// Remove their "ball hit paddle" effect
		if c.Shimmering[1] != 0 {
			c.Shimmering[1]--
		}

		// Move paddle to follow ball
		if c.BallPos.X > -c.Player[1].X {
			c.Player[1].X -= (c.ComputerSpeed + float64(rand.Intn(2)))
		} else if c.BallPos.X < -c.Player[1].X {
			c.Player[1].X += (c.ComputerSpeed + float64(rand.Intn(2)))
		}

		if c.BallPos.Y < c.Player[1].Y {
			c.Player[1].Y -= (c.ComputerSpeed + float64(rand.Intn(2)))
		} else if c.BallPos.Y > c.Player[1].Y {
			c.Player[1].Y += (c.ComputerSpeed + float64(rand.Intn(2)))
		}

		// Launch ball if it's our serve
		if !c.BallInPlay && c.BallWaitingFor == 1 && rand.Intn(10) < 1 {
			c.putBallInPlay(1)
		}
	}
}

func (c *Game) moveDebris() {
	// Move debris
	for i := range c.Debris {
		d := &c.Debris[i]
		if d.Exist {
			d.Pos = vec3.Add(d.Pos, d.Vel)

			// Add effect of gravity
			if c.Mode != HANDBALL {
				d.Vel.Y += c.Gravity / 2
			} else {
				d.Vel.Z -= c.Gravity / 2
			}

			// Count it down, remove if it's old
			if d.Time--; d.Time <= 0 {
				d.Exist = false
			}
		}
	}
}

func (c *Game) moveCrap() {
	// Move crap
	c.CrapPos = vec3.Add(c.CrapPos, c.CrapVel)
	if c.CrapPos.X < -c.Arena.X {
		c.CrapPos.X = -c.Arena.X
		c.CrapVel = ga.Vec3d{0, 0, 1}
	} else if c.CrapPos.X > c.Arena.X {
		c.CrapPos.X = c.Arena.X
		c.CrapVel = ga.Vec3d{0, 0, -1}
	}

	if c.CrapPos.Z < -c.Arena.Z {
		c.CrapPos.Z = -c.Arena.Z
		c.CrapVel = ga.Vec3d{-1, 0, 0}
	} else if c.CrapPos.Z > c.Arena.Z {
		c.CrapPos.Z = c.Arena.Z
		c.CrapVel = ga.Vec3d{1, 0, 0}
	}
}

func (c *Game) moveBall() {
	// Move ball
	if !c.BallInPlay {
		return
	}

	// Move it left/right
	c.BallPos.X += c.BallVel.X

	if c.BallPos.X < -c.Arena.X+c.BallSize {
		c.BallPos.X = -c.Arena.X + c.BallSize
		c.BallVel.X = -c.BallVel.X
		c.playSound("wall")
	} else if c.BallPos.X > c.Arena.X-c.BallSize {
		c.BallPos.X = c.Arena.X - c.BallSize
		c.BallVel.X = -c.BallVel.X
		c.playSound("wall")
	}

	// Move it up/down
	c.BallPos.Y += c.BallVel.Y

	if c.BallPos.Y < -c.Arena.Y+c.BallSize {
		c.BallPos.Y = -c.Arena.Y + c.BallSize
		c.BallVel.Y = -c.BallVel.Y
		c.playSound("wall")
	} else if c.BallPos.Y > c.Arena.Y-c.BallSize {
		c.BallPos.Y = c.Arena.Y - c.BallSize
		c.BallVel.Y = -c.BallVel.Y
		c.playSound("wall")
	}

	// Add the effect of gravity
	if c.Mode != HANDBALL {
		c.BallVel.Y += c.Gravity
	} else {
		c.BallVel.Z -= c.Gravity
	}

	// Move it in/out
	c.BallPos.Z += c.BallVel.Z

	// It's at a goal!
	if c.BallPos.Z < -c.Arena.Z+c.BallSize {
		if c.BallPos.X+c.BallSize >= c.Player[0].X-c.PaddleSize.X &&
			c.BallPos.X-c.BallSize <= c.Player[0].X+c.PaddleSize.X &&
			c.BallPos.Y+c.BallSize >= c.Player[0].Y-c.PaddleSize.Y &&
			c.BallPos.Y-c.BallSize <= c.Player[0].Y+c.PaddleSize.Y {

			// They hit it! Bounce!
			c.addDebris()
			c.playSound("hit")

			c.BallSpeed += 1
			c.BallPos.Z = -c.Arena.Z + c.BallSize
			c.BallVel.Z = float64(rand.Intn(int(c.BallSpeed/2))) + float64(c.BallSpeed)/2

			c.addDebris()

			if c.Spin == 0 {
				c.BallVel.X = (c.BallPos.X - c.Player[0].X) / c.AngleDivide
				c.BallVel.Y = (c.BallPos.Y - c.Player[0].Y) / c.AngleDivide
			} else if c.Spin == 1 {
				total := c.total(c.Queue[1])
				c.BallVel.X = total.X
				c.BallVel.Y = total.Y
			}

			c.Shimmering[0] = c.ShimmerTime

			// A hit in handball mode means score
			if c.Mode == HANDBALL {
				c.Score[0]++
				c.FinalScore = c.Score[0]

				if c.Score[0] > c.HighScore {
					c.HighScore = c.Score[0]
				}
			}
		} else if c.BallPos.Z <= -c.Arena.Z {
			if c.Mode != HANDBALL {
				// They missed it!  Score to player 2!
				c.playSound("score")
				c.BallInPlay = false
				c.BallWaitingFor = 1
				c.Score[1]++
			} else {
				c.FinalScore = c.Score[0]
				c.GotHighScore = false
				if c.FinalScore >= c.HighScore {
					c.GotHighScore = true
				}

				c.BallInPlay = false
				c.BallWaitingFor = 0
				c.Score[0] = 0
			}
		}
	} else if c.BallPos.Z > c.Arena.Z-c.BallSize {
		if c.Mode != HANDBALL {
			if c.BallPos.X+c.BallSize >= -c.Player[1].X-c.PaddleSize.X &&
				c.BallPos.X-c.BallSize <= -c.Player[1].X+c.PaddleSize.X &&
				c.BallPos.Y+c.BallSize >= c.Player[1].Y-c.PaddleSize.Y &&
				c.BallPos.Y-c.BallSize <= c.Player[1].Y+c.PaddleSize.Y {

				// They hit it!  Bounce!
				c.addDebris()
				c.playSound("hit")

				c.BallSpeed += 1
				c.BallPos.Z = c.Arena.Z - c.BallSize
				c.BallVel.Z = float64(-rand.Intn(int(c.BallSpeed/2))) - c.BallSpeed/2

				c.addDebris()

				if c.Spin == 0 {
					c.BallVel.X = (c.BallPos.X - c.Player[1].X) / c.AngleDivide
					c.BallVel.Y = (c.BallPos.Y - c.Player[1].Y) / c.AngleDivide
				} else if c.Spin == 1 {
					total := c.total(c.Queue[1])
					c.BallVel.X = total.X
					c.BallVel.Y = total.Y
				}

				c.Shimmering[1] = c.ShimmerTime
			}
		} else {
			c.BallPos.Z = c.Arena.Z - c.BallSize
			c.BallVel.Z = -c.BallVel.Z
			c.playSound("wall")
		}
	}

	// Bounce ball of net
	if c.NetHeight != 0 {
		if c.BallPos.Z >= -c.BallSize && c.BallPos.Z <= c.BallSize && c.BallPos.Y >= c.NetHeight-c.BallSize {
			c.playSound("wall")
			c.BallVel.Z = -c.BallVel.Z
			c.BallPos.Z += c.BallVel.Z
		}
	}
}

func (c *Game) total(queue []ga.Vec2d) ga.Vec2d {
	var v ga.Vec2d
	for _, q := range queue {
		v.X += q.X
		v.Y += q.Y
	}
	v.X /= float64(len(queue))
	v.Y /= float64(len(queue))
	return v
}

func (c *Game) playSound(snd string) {
	if !c.Sound {
		return
	}
	c.Sfx[snd].PlayChannel(1, 0)
}

func (c *Game) draw() {
	re := c.Renderer
	re.SetDrawColor(sdlcolor.Black)
	re.Clear()
	plns := 1
	if c.Mode == TWO_PLAYERS {
		plns = 2
	}
	for pln := 0; pln < plns; pln++ {
		c.drawArena(pln)
		c.drawFloorMarker(pln)
		c.drawOpponent(pln)
		c.drawBall(pln)
		c.drawYou(pln)
		c.drawDebris(pln)
		c.drawViewMode(pln)
		c.drawScores(pln)
		c.drawPause(pln)
	}
	re.Present()
}

func (c *Game) drawPause(pln int) {
	if !c.Pause {
		return
	}
	r := &c.Bound[pln]
	x := r.Dx()/2 - 40
	y := r.Dy() / 2
	c.drawText(pln, x, y, sdlcolor.White, "PAUSED")
}

func (c *Game) drawArena(pln int) {
	x := c.Arena.X
	y := c.Arena.Y
	z := c.Arena.Z
	col := sdlcolor.White

	// Draw game arena
	c.drawLine(
		pln,
		ga.Vec3d{-x, -y, -z},
		ga.Vec3d{-x, -y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, -y, -z},
		ga.Vec3d{+x, -y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x, +y, -z},
		ga.Vec3d{-x, +y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, +y, -z},
		ga.Vec3d{+x, +y, +z},
		col,
	)

	c.drawLine(
		pln,
		ga.Vec3d{-x, -y, -z},
		ga.Vec3d{+x, -y, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, -y, -z},
		ga.Vec3d{+x, +y, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, +y, -z},
		ga.Vec3d{-x, +y, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x, +y, -z},
		ga.Vec3d{-x, -y, -z},
		col,
	)

	c.drawLine(
		pln,
		ga.Vec3d{-x, -y, +z},
		ga.Vec3d{+x, -y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, -y, +z},
		ga.Vec3d{+x, +y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{+x, +y, +z},
		ga.Vec3d{-x, +y, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x, +y, +z},
		ga.Vec3d{-x, -y, +z},
		col,
	)
}

func (c *Game) drawFloorMarker(pln int) {
	if c.Mode != HANDBALL {
		// Draw floor marker
		x := c.Arena.X
		y := c.Arena.Y
		col := sdlcolor.White

		c.drawLine(
			pln,
			ga.Vec3d{-x, +y, 0},
			ga.Vec3d{+x, +y, 0},
			col,
		)

		// Draw net, if any
		if c.Net != 0 {
			ny := c.NetHeight
			c.drawLine(
				pln,
				ga.Vec3d{-x, +ny, 0},
				ga.Vec3d{+x, +ny, 0},
				col,
			)
			c.drawLine(
				pln,
				ga.Vec3d{+x, +y, 0},
				ga.Vec3d{+x, +ny, 0},
				col,
			)
			c.drawLine(
				pln,
				ga.Vec3d{-x, +ny, 0},
				ga.Vec3d{-x, +y, 0},
				col,
			)
		}
	}
}

func (c *Game) drawOpponent(pln int) {
	// Draw opponent
	if c.Mode == HANDBALL {
		return
	}
	x := c.Player[1-pln].X
	y := c.Player[1-pln].Y
	z := c.Arena.Z

	pw := c.PaddleSize.X
	ph := c.PaddleSize.Y

	col := c.Colors[pln][1-pln+3]

	c.drawLine(
		pln,
		ga.Vec3d{-x + pw, +y - ph, +z},
		ga.Vec3d{-x - pw, +y - ph, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x - pw, +y - ph, +z},
		ga.Vec3d{-x - pw, +y + ph, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x - pw, +y + ph, +z},
		ga.Vec3d{-x + pw, +y + ph, +z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{-x + pw, +y + ph, +z},
		ga.Vec3d{-x + pw, +y - ph, +z},
		col,
	)

	// Draw "paddle hit the ball" effect
	if c.Shimmering[1-pln] != 0 {
		col := c.Colors[pln][1-pln+3]
		c.drawLine(
			pln,
			ga.Vec3d{-x - pw, y - ph, z},
			ga.Vec3d{-x + pw, y + ph, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{-x + pw, y - ph, z},
			ga.Vec3d{-x - pw, y + ph, z},
			col,
		)
	}
}

func (c *Game) drawBall(pln int) {
	if c.BallInPlay {
		// Draw ball
		var x, z float64
		if pln == 0 {
			x = c.BallPos.X
			z = c.BallPos.Z
		} else {
			x = -c.BallPos.X
			z = -c.BallPos.Z
		}
		y := c.BallPos.Y
		s := c.BallSize
		col := c.Colors[pln][2]

		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z - s},
			ga.Vec3d{x + s, y, z - s},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z - s},
			ga.Vec3d{x + s, y, z + s},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z + s},
			ga.Vec3d{x - s, y, z + s},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z + s},
			ga.Vec3d{x - s, y, z - s},
			col,
		)

		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z - s},
			ga.Vec3d{x, y - s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z - s},
			ga.Vec3d{x, y - s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z + s},
			ga.Vec3d{x, y - s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z + s},
			ga.Vec3d{x, y - s, z},
			col,
		)

		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z - s},
			ga.Vec3d{x, y + s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z - s},
			ga.Vec3d{x, y + s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + s, y, z + s},
			ga.Vec3d{x, y + s, z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x - s, y, z + s},
			ga.Vec3d{x, y + s, z},
			col,
		)

		// Draw ball markers
		col = c.Colors[pln][5]
		y = -c.Arena.Y
		c.drawLine(
			pln,
			ga.Vec3d{x - s, -y, z},
			ga.Vec3d{x + s, -y, z},
			col,
		)

		x = c.Arena.X
		y = c.BallPos.Y
		c.drawLine(
			pln,
			ga.Vec3d{-x, y - s, z},
			ga.Vec3d{-x, y + s, z},
			col,
		)
	} else {
		// Ball isn't in play, waiting for someone...
		var text string
		if c.BallWaitingFor == pln {
			text = fmt.Sprintf("Your serve!")
		} else {
			text = fmt.Sprintf("Player %d's serve", (1-pln)+1)
		}

		c.drawText(pln, 50, c.Height/2, sdlcolor.White, text)

		// Show final score (handball)
		if c.Mode == HANDBALL {
			fh := c.FontHeight
			// Only show it if they've actually played a round yet
			if c.FinalScore != -1 {
				text = fmt.Sprintf("Final score: %d", c.FinalScore)
				c.drawText(pln, 50, int(c.Height)/2+fh*2, sdlcolor.White, text)
			}

			// Show "got high score" if they got it (handball)
			if c.GotHighScore {
				c.drawText(pln, 50, int(c.Height)/2+fh*3, sdlcolor.White, "You beat the high score!")
			}
		}
	}
}

func (c *Game) drawYou(pln int) {
	x := c.Player[pln].X
	y := c.Player[pln].Y
	z := c.Arena.Z
	pw := c.PaddleSize.X
	ph := c.PaddleSize.Y
	col := c.Colors[pln][pln]

	c.drawLine(
		pln,
		ga.Vec3d{x - pw, y - ph, -z},
		ga.Vec3d{x + pw, y - ph, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{x + pw, y - ph, -z},
		ga.Vec3d{x + pw, y + ph, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{x + pw, y + ph, -z},
		ga.Vec3d{x - pw, y + ph, -z},
		col,
	)
	c.drawLine(
		pln,
		ga.Vec3d{x - pw, y + ph, -z},
		ga.Vec3d{x - pw, y - ph, -z},
		col,
	)

	// Draw "paddle hit the ball" effect
	if c.Shimmering[pln] != 0 {
		c.Shimmering[pln]--

		c.drawLine(
			pln,
			ga.Vec3d{x - pw, y - ph, -z},
			ga.Vec3d{x + pw, y + ph, -z},
			col,
		)
		c.drawLine(
			pln,
			ga.Vec3d{x + pw, y - ph, -z},
			ga.Vec3d{x - pw, y + ph, -z},
			col,
		)
	}
}

func (c *Game) drawDebris(pln int) {
	for i := range c.Debris {
		d := &c.Debris[i]
		if d.Exist {
			col := c.Colors[pln][rand.Intn(3)]
			c.drawLine(pln, d.Pos, vec3.Add(d.Pos, d.Vel), col)
		}
	}
}

func (c *Game) drawViewMode(pln int) {
	// Draw view mode
	x := 10
	fh := c.FontHeight
	viewNames := [...]string{
		"Normal", "Bleachers", "Above", "FreeView",
		"Follow the Ball", "From The Paddle",
	}
	c.drawText(pln, x, c.Height-fh, c.Colors[pln][2], viewNames[c.View[pln]])
	if c.View[pln] == 3 {
		c.drawText(pln, x, c.Height, c.Colors[pln][5], "Middle-Click and drag to change view")
	}
}

func (c *Game) drawScores(pln int) {
	fh := c.FontHeight
	x := 10
	// Draw scores
	if c.Mode != HANDBALL {
		// Player 1 and 2 scores
		for i := 0; i < 2; i++ {
			t := [2]int{1, 2}
			if pln == 1 {
				t = [2]int{2, 1}
			}
			c.drawText(pln, x, fh*(i+1), c.Colors[pln][i], "Player %d: %d", t[i], c.Score[t[i]-1])
		}
	} else {
		// Score and high score for handball
		c.drawText(pln, x, fh, c.Colors[0][0], "Score: %d", c.Score[0])
		c.drawText(pln, x, fh*2, c.Colors[0][0], "High:  %d", c.HighScore)
	}
}

func (c *Game) drawText(pln, x, y int, col color.RGBA, format string, args ...interface{}) {
	x += c.Bound[pln].Min.X
	text := fmt.Sprintf(format, args...)
	texture := c.Texture
	surface := c.Surface
	font := c.Font

	r, err := font.RenderUTF8BlendedEx(surface, text, col)
	ck(err)

	p, err := texture.Lock(nil)
	ck(err)

	err = surface.Lock()
	ck(err)
	s := surface.Pixels()
	for i := 0; i < len(p); i += 4 {
		p[i] = s[i+2]
		p[i+1] = s[i+1]
		p[i+2] = s[i]
		p[i+3] = s[i+3]
	}

	surface.Unlock()
	texture.Unlock()

	texture.SetBlendMode(sdl.BLENDMODE_BLEND)
	c.Renderer.Copy(texture, &sdl.Rect{0, 0, r.W, r.H}, &sdl.Rect{int32(x), int32(y), r.W, r.H})
}

func (c *Game) drawLine(pln int, p1, p2 ga.Vec3d, col color.RGBA) {
	re := c.Renderer

	var (
		xoff     float64
		pp1, pp2 ga.Vec3d
		s1, s2   ga.Vec2d
	)
	for i := 0; i < c.Glasses[pln]+1; i++ {
		if c.Glasses[pln] == 0 {
			xoff = 0
		} else if c.Glasses[pln] == 1 {
			xoff = c.GlassOffset

			if i == 0 {
				xoff = -xoff
			}
			if !c.Toggle {
				xoff = -xoff
			}
		}

		// Alter perceived x/y/z depending on their view
		switch c.View[pln] {
		case 0: // Normal (behind your paddle)
			pp1 = p1
			pp2 = p2

		case 1: // From the side
			pp1 = ga.Vec3d{p1.Z, p1.Y, p1.X}
			pp2 = ga.Vec3d{p2.Z, p2.Y, p2.X}

		case 2: // From above
			pp1 = ga.Vec3d{p1.X, p1.Z, p1.Y}
			pp2 = ga.Vec3d{p2.X, p2.Z, p2.Y}

		case 3: // Free view
			xx1 := p1.X*c.CosAngle[pln].X - p1.Z*c.SinAngle[pln].X
			zz1 := p1.X*c.SinAngle[pln].X + p1.Z*c.CosAngle[pln].X

			yy1 := p1.Y*c.CosAngle[pln].Y - zz1*c.SinAngle[pln].Y
			zz1 = p1.Y*c.SinAngle[pln].Y + zz1*c.CosAngle[pln].Y

			xx2 := p2.X*c.CosAngle[pln].X - p2.Z*c.SinAngle[pln].X
			zz2 := p2.X*c.SinAngle[pln].X + p2.Z*c.CosAngle[pln].X

			yy2 := p2.Y*c.CosAngle[pln].Y - zz2*c.SinAngle[pln].Y
			zz2 = p2.Y*c.SinAngle[pln].Y + zz2*c.CosAngle[pln].Y

			pp1 = ga.Vec3d{xx1, yy1, zz1}
			pp2 = ga.Vec3d{xx2, yy2, zz2}

		case 4: // Watch the ball
			ball := &c.BallPos
			anglex := (ball.Z - c.Arena.Z) / 10
			anglex = ga.Clamp(anglex, -90, 90)

			anglex = (anglex / 180) * math.Pi
			sinx, cosx := math.Sincos(anglex)

			xx1 := p1.X - (ball.X / 5)
			yy1 := (p1.Z-ball.Z)*sinx + (p1.Y-ball.Y)*cosx
			zz1 := (p1.Z-ball.Z)*cosx - (p1.Y-ball.Y)*sinx + 30

			xx2 := p2.X - (ball.X / 5)
			yy2 := (p2.Z-ball.Z)*sinx + (p2.Y-ball.Y)*cosx
			zz2 := (p2.Z-ball.Z)*cosx - (p2.Y-ball.Y)*sinx + 30

			pp1 = ga.Vec3d{xx1, yy1, zz1}
			pp2 = ga.Vec3d{xx2, yy2, zz2}

		case 5: // From your paddle
			pp1 = ga.Vec3d{
				p1.X - c.Player[pln].X,
				p1.Y - c.Player[pln].Y,
				p1.Z,
			}

			pp2 = ga.Vec3d{
				p2.X - c.Player[pln].X,
				p2.Y - c.Player[pln].Y,
				p2.Z,
			}
		}

		// Inside of the distance clip plane
		if pp1.Z > -c.Distance && pp2.Z > -c.Distance {
			// Convert (x,y,z) into (x,y) with a 3D look;
			s1 = ga.Vec2d{
				(pp1.X + xoff) / ((pp1.Z + c.Distance) / c.Aspect),
				pp1.Y / ((pp1.Z + c.Distance) / c.Aspect),
			}

			s2 = ga.Vec2d{
				(pp2.X + xoff) / ((pp2.Z + c.Distance) / c.Aspect),
				pp2.Y / ((pp2.Z + c.Distance) / c.Aspect),
			}

			// Transpose (0, 0) origin to center of window
			r := &c.Bound[pln]
			width := r.Dx()
			height := r.Dy()

			s1.X += float64(r.Min.X)
			s1.X += float64(width) / 2
			s1.Y += float64(height) / 2

			s2.X += float64(r.Min.X)
			s2.X += float64(width) / 2
			s2.Y += float64(height) / 2

			// Draw the line into the window
			if c.Glasses[pln] == 0 {
				re.SetDrawColor(col)
				re.DrawLine(int(s1.X), int(s1.Y), int(s2.X), int(s2.Y))
			} else {
				if (i == 0 && c.Toggle) || (i == 1 && !c.Toggle) {
					re.SetDrawColor(c.RedBlue[pln][0])
					re.DrawLine(int(s1.X), int(s1.Y), int(s2.X), int(s2.Y))
				} else {
					re.SetDrawColor(c.RedBlue[pln][1])
					re.DrawLine(int(s1.X), int(s1.Y+1), int(s2.X), int(s2.Y+1))
				}
			}
		}
	}
}

func (c *Game) addDebris() {
	n := rand.Intn(c.DebrisMax-c.DebrisMin) + c.DebrisMin
	for i := 0; i < n; i++ {
		d := &c.Debris[c.DebrisCount]

		// Make it exist
		d.Exist = true
		d.Time = rand.Intn(c.DebrisTime)

		// Give it a position
		p := &d.Pos
		v := &c.BallPos
		p.X = v.X + float64(rand.Intn(10)) - 5
		p.Y = v.Y + float64(rand.Intn(10)) - 5
		p.Z = v.Z + float64(rand.Intn(10)) - 5

		// Give it a speed/direction
		p = &d.Vel
		v = &c.BallVel
		p.X = (float64(rand.Intn(c.DebrisSpeed*2)) - float64(c.DebrisSpeed) + v.X/2)
		p.Y = 0
		p.Z = 0

		// Increment the debris counter
		c.DebrisCount = (c.DebrisCount + 1) % len(c.Debris)
	}
}

func (c *Game) putBallInPlay(pln int) {
	// Remember that the ball is now in play
	c.BallInPlay = true

	// Pick a random starting position
	c.BallPos = ga.Vec3d{
		c.Player[pln].X,
		c.Player[pln].Y,
		c.Arena.Z / 2,
	}

	if pln == 0 {
		c.BallPos.X = -c.BallPos.X
		c.BallPos.Z = -c.BallPos.Z
	}

	// Give it a random speed/direction
	c.BallSpeed = c.InitialBallSpeed
	c.BallVel.X = float64(rand.Intn(int(c.BallSpeed*2))) - c.BallSpeed
	c.BallVel.Y = float64(rand.Intn(int(c.BallSpeed*2))) - c.BallSpeed
	for {
		c.BallVel.Z = float64(rand.Intn(int(c.InitialBallSpeed*3))) / 2
		if c.BallVel.Z != 0 {
			break
		}
	}

	if pln == 1 {
		c.BallVel.Z = -c.BallVel.Z
	}
}
