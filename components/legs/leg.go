package legs

import (
	"fmt"
	"github.com/adammck/dynamixel"
	"github.com/adammck/hexapod/math3d"
	"github.com/adammck/hexapod/utils"
	"math"
)

type Leg struct {
	Origin *math3d.Vector3

	// TODO: Rename this to 'Heading', since that's what it is
	Angle float64

	Name   string
	Coxa   *dynamixel.DynamixelServo
	Femur  *dynamixel.DynamixelServo
	Tibia  *dynamixel.DynamixelServo
	Tarsus *dynamixel.DynamixelServo

	// Has the leg been initialized yet? It can't be moved until it has.
	Initialized bool
}

func NewLeg(network *dynamixel.DynamixelNetwork, baseId int, name string, origin *math3d.Vector3, angle float64) *Leg {
	return &Leg{
		Origin:      origin,
		Angle:       angle,
		Name:        name,
		Coxa:        dynamixel.NewServo(network, uint8(baseId+1)),
		Femur:       dynamixel.NewServo(network, uint8(baseId+2)),
		Tibia:       dynamixel.NewServo(network, uint8(baseId+3)),
		Tarsus:      dynamixel.NewServo(network, uint8(baseId+4)),
		Initialized: false,
	}
}

// Matrix returns a pointer to a 4x4 matrix, to transform a vector in the leg's
// coordinate space into the parent (hexapod) space.
func (leg *Leg) Matrix() math3d.Matrix44 {
	return *math3d.MakeMatrix44(*leg.Origin, *math3d.MakeSingularEulerAngle(math3d.RotationHeading, leg.Angle))
}

// Servos returns an array of all servos attached to this leg.
func (leg *Leg) Servos() [4]*dynamixel.DynamixelServo {
	return [4]*dynamixel.DynamixelServo{
		leg.Coxa,
		leg.Femur,
		leg.Tibia,
		leg.Tarsus,
	}
}

func (leg *Leg) SetLED(state bool) {
	for _, s := range leg.Servos() {
		s.SetLed(state)
	}
}

// http://en.wikipedia.org/wiki/Solution_of_triangles#Three_sides_given_.28SSS.29
func _sss(a float64, b float64, c float64) float64 {
	return utils.Deg(math.Acos(((b * b) + (c * c) - (a * a)) / (2 * b * c)))
}

func (leg *Leg) segments() (*Segment, *Segment, *Segment, *Segment) {

	// The position of the object in space must be specified by two segments. The
	// first positions it, then the second (which is always zero-length) rotates
	// it into the home orientation.
	r1 := MakeRootSegment(*math3d.MakeVector3(leg.Origin.X, leg.Origin.Y, leg.Origin.Z))
	r2 := MakeSegment("r2", r1, *math3d.MakeSingularEulerAngle(math3d.RotationHeading, leg.Angle), *math3d.MakeVector3(0, 0, 0))

	// Movable segments (angles in deg, vectors in mm)
	coxa := MakeSegment("coxa", r2, *math3d.MakeSingularEulerAngle(math3d.RotationHeading, 40), *math3d.MakeVector3(39, -12, 0))
	femur := MakeSegment("femur", coxa, *math3d.MakeSingularEulerAngle(math3d.RotationBank, 90), *math3d.MakeVector3(100, 0, 0))
	tibia := MakeSegment("tibia", femur, *math3d.MakeSingularEulerAngle(math3d.RotationBank, 0), *math3d.MakeVector3(85, 0, 0))
	tarsus := MakeSegment("tarsus", tibia, *math3d.MakeSingularEulerAngle(math3d.RotationBank, 90), *math3d.MakeVector3(76.5, 0, 0))

	// Return just the useful segments
	return coxa, femur, tibia, tarsus
}

// Sets the goal position of this leg to the given x/y/z coordinates, relative
// to the center of the hexapod.
func (leg *Leg) SetGoal(p math3d.Vector3) {
	_, femur, _, _ := leg.segments()

	// TODO (adammck): Return an error instead!
	if !leg.Initialized {
		panic("leg not initialized")
	}

	v := &math3d.Vector3{p.X, p.Y, p.Z}
	vv := v.Add(math3d.Vector3{0, 64, 0})

	// Solve the angle of the coxa by looking at the position of the target from
	// above (x,z). It's the only joint which rotates around the Y axis, so we can
	// cheat.

	adj := v.X - leg.Origin.X
	opp := v.Z - leg.Origin.Z
	theta := utils.Deg(math.Atan2(-opp, adj))
	coxaAngle := (theta - leg.Angle)

	// Solve the other joints with a bunch of trig. Since we've already set the Y
	// rotation and the other joints only rotate around X (relative to the coxa,
	// anyway), we can solve them with a shitload of triangles.

	r := femur.Start()
	t := r
	t.Y = -50

	a := 100.0 // femur length
	b := 85.0  // tibia length
	c := 64.0  // tarsus length
	d := r.Distance(*vv)
	e := r.Distance(*v)
	f := r.Distance(t)
	g := t.Distance(*v)

	aa := _sss(b, a, d)
	bb := _sss(c, d, e)
	cc := _sss(g, e, f)
	dd := _sss(a, d, b)
	ee := _sss(e, c, d)
	hh := 180 - aa - dd

	femurAngle := (aa + bb + cc) - 90
	tibiaAngle := 180 - hh
	tarsusAngle := 180 - (dd + ee)

	// fmt.Printf("v=%v, vv=%v, r=%v, t=%v\n", v, vv, r, t)
	// fmt.Printf("a=%0.4f, b=%0.4f, c=%0.4f, d=%0.4f, e=%0.4f, f=%0.4f, g=%0.4f\n", a, b, c, d, e, f, g)
	// fmt.Printf("aa=%0.4f, bb=%0.4f, cc=%0.4f, dd=%0.4f, ee=%0.4f\n", aa, bb, cc, dd, ee)
	// fmt.Printf("coxaAngle=%0.4f (s/o=%0.4f) (s/v=%0.4f) (e/o=%0.4f) (e/v=%0.4f)\n", coxaAngle, coxa.Start().Distance(ik.ZeroVector3), coxa.Start().Distance(*v), coxa.End().Distance(ik.ZeroVector3), coxa.End().Distance(*v))
	// fmt.Printf("femurAngle=%0.4f (s/o=%0.4f) (s/v=%0.4f) (e/o=%0.4f) (e/v=%0.4f)\n", femurAngle, femur.Start().Distance(ik.ZeroVector3), femur.Start().Distance(*v), femur.End().Distance(ik.ZeroVector3), femur.End().Distance(*v))
	// fmt.Printf("tibiaAngle=%0.4f (s/o=%0.4f) (s/v=%0.4f) (e/o=%0.4f) (e/v=%0.4f)\n", tibiaAngle, tibia.Start().Distance(ik.ZeroVector3), tibia.Start().Distance(*v), tibia.End().Distance(ik.ZeroVector3), tibia.End().Distance(*v))
	// fmt.Printf("tarsusAngle=%0.4f (s/o=%0.4f) (s/v=%0.4f) (e/o=%0.4f) (e/v=%0.4f)\n", tarsusAngle, tarsus.Start().Distance(ik.ZeroVector3), tarsus.Start().Distance(*v), tarsus.End().Distance(ik.ZeroVector3), tarsus.End().Distance(*v))

	if math.IsNaN(coxaAngle) || math.IsNaN(femurAngle) || math.IsNaN(tibiaAngle) || math.IsNaN(tarsusAngle) {
		fmt.Println("ERROR")
		return
	}

	leg.Coxa.MoveTo(coxaAngle)
	leg.Femur.MoveTo(0 - femurAngle)
	leg.Tibia.MoveTo(tibiaAngle)
	leg.Tarsus.MoveTo(tarsusAngle)
}
