package stops

import (
	"context"
	"log/slog"
	"math"
	"testing"

	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/stay"
)

var (
	samara     = road.Point{Lat: 53.2038, Lon: 50.1606}
	yoshkarOla = road.Point{Lat: 56.6344, Lon: 47.8999}

	kazan           = stay.MapCity{ID: "Q900", Name: "Казань", Pop: 1329825, Lat: 55.7879, Lon: 49.1233, Arms: true, Photo: "k.jpg", Landmark: "Кремль"}
	cheboksary      = stay.MapCity{ID: "Q5470", Name: "Чебоксары", Pop: 497061, Lat: 56.1439, Lon: 47.2489, Arms: true}
	ulyanovsk       = stay.MapCity{ID: "Q5627", Name: "Ульяновск", Pop: 624518, Lat: 54.3142, Lon: 48.4031, Arms: true}
	zelenodolsk     = stay.MapCity{ID: "Q1", Name: "Зеленодольск", Pop: 98000, Lat: 55.8466, Lon: 48.5010}
	dimitrovgrad    = stay.MapCity{ID: "Q4", Name: "Димитровград", Pop: 113000, Lat: 54.2167, Lon: 49.6167}
	moscow          = stay.MapCity{ID: "Q649", Name: "Москва", Pop: 13000000, Lat: 55.7558, Lon: 37.6173, Arms: true}
	novokuybyshevsk = stay.MapCity{ID: "Q2", Name: "Новокуйбышевск", Pop: 100000, Lat: 53.0959, Lon: 49.9462}
	almaty          = stay.MapCity{ID: "Q35493", Name: "Алматы", Country: "Казахстан", Pop: 2000000, Lat: 55.8, Lon: 49.2}
)

// Дорога Самара → Йошкар-Ола идёт через Казань (по левому берегу Волги, мимо Димитровграда)
var viaKazan = []road.Point{samara, pt(kazan), yoshkarOla}

func TestCandidatesAlongTheRoad(t *testing.T) {
	got := Candidates([]stay.MapCity{kazan, cheboksary, ulyanovsk, moscow, novokuybyshevsk, zelenodolsk, dimitrovgrad}, viaKazan)
	ids := map[string]float64{}
	for _, c := range got {
		ids[c.City.ID] = c.Pos
	}
	for _, want := range []string{"Q900", "Q1", "Q4"} {
		if _, ok := ids[want]; !ok {
			t.Errorf("нет %s среди кандидатов: %v", want, ids)
		}
	}
	// Чебоксары и Ульяновск — за Волгой, в стороне от этой дороги; Москва — совсем в другой стороне
	for _, no := range []string{"Q5470", "Q5627", "Q649"} {
		if _, ok := ids[no]; ok {
			t.Errorf("%s в стороне от дороги — не кандидат", no)
		}
	}
	if _, ok := ids["Q2"]; ok {
		t.Error("Новокуйбышевск у самого дома — не остановка")
	}
	// Положение — вдоль дороги: Димитровград раньше Казани, Зеленодольск — после
	if !(ids["Q4"] < ids["Q900"] && ids["Q900"] < ids["Q1"]) {
		t.Errorf("порядок вдоль дороги: %v", ids)
	}
}

func TestCandidatesShortRoute(t *testing.T) {
	if got := Candidates([]stay.MapCity{kazan}, []road.Point{pt(kazan), pt(zelenodolsk)}); got != nil {
		t.Errorf("на коротком пути остановок нет, got %v", got)
	}
}

func TestNearest(t *testing.T) {
	// Точка в 1° широты к северу от середины отрезка вдоль экватора: ~110 км в сторону, ~111 км вдоль
	off, along := nearest([]road.Point{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 2}}, road.Point{Lat: 1, Lon: 1})
	if math.Abs(off-110.6) > 1 || math.Abs(along-111.3) > 1 {
		t.Errorf("off = %.1f, along = %.1f", off, along)
	}
}

func pt(c stay.MapCity) road.Point { return road.Point{Lat: c.Lat, Lon: c.Lon} }

func hours(h float64) road.Leg { return road.Leg{DurationS: int(h * 3600), DistanceM: int(h * 80000)} }

func TestRate(t *testing.T) {
	direct := hours(7)
	// Казань: 4,5 ч + 3 ч — крюк 30 минут, ближе к середине
	sg, ok := Rate(kazan, hours(4.5), hours(3), direct, 8)
	if !ok {
		t.Fatal("крюк в полчаса допустим")
	}
	if sg.Detour.DurationS != 1800 || sg.Overnight {
		t.Errorf("Казань при лимите 8 ч: %+v", sg)
	}
	if sg.Position < 0.59 || sg.Position > 0.61 {
		t.Errorf("position = %v, want 0.6", sg.Position)
	}
	// Крюк в 2 часа на 7-часовой дороге — слишком много
	if _, ok := Rate(cheboksary, hours(5), hours(4), direct, 8); ok {
		t.Error("крюк 2 ч не предлагаем")
	}
}

func TestRateOvernight(t *testing.T) {
	direct := hours(12)
	// Лимит 8 ч: посередине — оба участка в лимите, это ночёвка
	mid, _ := Rate(kazan, hours(6), hours(6.2), direct, 8)
	if !mid.Overnight {
		t.Errorf("середина длинной дороги — ночёвка: %+v", mid)
	}
	// Слишком близко к дому: второй участок длиннее лимита
	early, _ := Rate(ulyanovsk, hours(2), hours(10.3), direct, 8)
	if early.Overnight {
		t.Errorf("второй день 10 ч — не ночёвка: %+v", early)
	}
	if early.Score >= mid.Score {
		t.Errorf("ночёвка в середине должна быть выше: %v >= %v", early.Score, mid.Score)
	}
}

func TestPickDedupes(t *testing.T) {
	list := []Suggestion{
		{City: zelenodolsk, Score: 4},
		{City: kazan, Score: 7},
		{City: stay.MapCity{ID: "Q3", Name: "Казань-2", Lat: 55.80, Lon: 49.15}, Score: 6}, // в 2 км от Казани
		{City: cheboksary, Score: 5},
	}
	got := Pick(list)
	if len(got) != 3 || got[0].City.ID != "Q900" || got[1].City.ID != "Q5470" || got[2].City.ID != "Q1" {
		t.Errorf("got %+v", got)
	}
}

type fakeRouter struct{ legs map[road.Point][]*road.Leg }

func (f fakeRouter) Table(_ context.Context, from road.Point, to []road.Point) ([]*road.Leg, error) {
	return f.legs[from][:len(to)], nil
}
func (f fakeRouter) Route(context.Context, road.Point, road.Point, ...road.Point) (*road.Route, error) {
	return nil, nil
}

func TestSuggestOnlyRussianCities(t *testing.T) {
	a, b := hours(4.5), hours(3)
	r := fakeRouter{legs: map[road.Point][]*road.Leg{samara: {&a}, yoshkarOla: {&b}}}
	s := NewService([]stay.MapCity{almaty, kazan}, r, nil, slog.Default())
	got, err := s.Suggest(context.Background(), samara, yoshkarOla, road.Route{Leg: hours(7), Polyline: road.EncodePolyline(viaKazan)}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].City.ID != "Q900" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := s.City("Q35493"); !ok {
		t.Error("зарубежный город остаётся в справочнике — по id его можно найти")
	}
}
