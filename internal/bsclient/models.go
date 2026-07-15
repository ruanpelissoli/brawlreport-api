package bsclient

// ─── Generic list wrapper ────────────────────────────────────────────────────

// ListResponse wraps any list endpoint response from the Brawl Stars API
// that uses the standard { "items": [...], "paging": {...} } envelope.
type ListResponse[T any] struct {
	Items  []T    `json:"items"`
	Paging Paging `json:"paging"`
}

// Paging carries cursor tokens returned by list endpoints.
type Paging struct {
	Cursors PagingCursors `json:"cursors"`
}

// PagingCursors holds before/after cursor strings for paginated requests.
type PagingCursors struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// ─── Player ──────────────────────────────────────────────────────────────────

// Player represents a Brawl Stars player profile returned by GET /players/{tag}.
type Player struct {
	Tag                                  string          `json:"tag"`
	Name                                 string          `json:"name"`
	NameColor                            string          `json:"nameColor"`
	Icon                                 PlayerIcon      `json:"icon"`
	Trophies                             int             `json:"trophies"`
	HighestTrophies                      int             `json:"highestTrophies"`
	ExpLevel                             int             `json:"expLevel"`
	ExpPoints                            int             `json:"expPoints"`
	IsQualifiedFromChampionshipChallenge bool            `json:"isQualifiedFromChampionshipChallenge"`
	ThreeVsThreeVictories                int             `json:"3vs3Victories"`
	SoloVictories                        int             `json:"soloVictories"`
	DuoVictories                         int             `json:"duoVictories"`
	BestRoboRumbleTime                   int             `json:"bestRoboRumbleTime"`
	BestTimeAsBigBrawler                 int             `json:"bestTimeAsBigBrawler"`
	Club                                 *PlayerClubStub `json:"club,omitempty"`
	Brawlers                             []PlayerBrawler `json:"brawlers"`
}

// PlayerIcon holds the numeric icon ID for the player's avatar.
type PlayerIcon struct {
	ID int `json:"id"`
}

// PlayerClubStub is the abbreviated club reference embedded in Player.
type PlayerClubStub struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

// PlayerBrawler is a brawler owned by the player, including progression data.
type PlayerBrawler struct {
	ID              int         `json:"id"`
	Name            string      `json:"name"`
	Power           int         `json:"power"`
	Rank            int         `json:"rank"`
	Trophies        int         `json:"trophies"`
	HighestTrophies int         `json:"highestTrophies"`
	Gears           []Gear      `json:"gears"`
	StarPowers      []StarPower `json:"starPowers"`
	Gadgets         []Gadget    `json:"gadgets"`
}

// Gear represents a gear attachment on a brawler.
type Gear struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// StarPower represents a star power unlocked on a brawler.
type StarPower struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Gadget represents a gadget unlocked on a brawler.
type Gadget struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ─── Battle log ──────────────────────────────────────────────────────────────

// BattleLogResponse is returned by GET /players/{tag}/battlelog.
// Items is a slice of BattleItem; this endpoint does NOT use the standard
// ListResponse envelope (items + paging, but no generic cursor use in practice).
type BattleLogResponse = ListResponse[BattleItem]

// BattleItem represents a single battle entry in the battlelog.
type BattleItem struct {
	BattleTime string      `json:"battleTime"`
	Event      BattleEvent `json:"event"`
	Battle     Battle      `json:"battle"`
}

// BattleEvent describes the game event (mode + map) the battle took place in.
type BattleEvent struct {
	ID   int    `json:"id"`
	Mode string `json:"mode"`
	Map  string `json:"map"`
}

// Battle holds the result data for a single match.
//
// Shape varies by mode — see field comments:
//
//   - 3v3 modes (gemGrab, brawlBall, bounty, heist, hotZone, knockout):
//     Teams is a 2-element slice (each element is a team slice of BattlePlayer).
//     Result and TrophyChange are present.
//
//   - Showdown solo: Players is present instead of Teams; Rank replaces Result.
//
//   - Showdown duo: Teams is present; Rank replaces Result; TrophyChange present.
//
//   - Friendly battles: Type == "friendly"; TrophyChange is nil — always exclude
//     friendly battles from win-rate statistics.
type Battle struct {
	Mode    string `json:"mode"`
	Type    string `json:"type"` // "ranked", "friendly", "soloRanked", "teamRanked", "challenge", etc.
	Duration int   `json:"duration"`

	// 3v3 result. Nil for Showdown modes.
	Result *string `json:"result,omitempty"` // "victory" | "defeat" | "draw"

	// Trophy change for this battle. Nil for friendly battles and modes where
	// trophies are not at stake. Downstream must treat nil as "no trophy change".
	TrophyChange *int `json:"trophyChange,omitempty"`

	// Rank is set for Showdown modes (1–10 solo, 1–5 duo). Nil otherwise.
	Rank *int `json:"rank,omitempty"`

	// StarPlayer is the MVP of the match (optional — absent in some modes).
	StarPlayer *BattlePlayer `json:"starPlayer,omitempty"`

	// Teams is used for 3v3 modes (2 teams × 3 players) and Showdown duo.
	// Nil for solo Showdown (use Players instead).
	Teams [][]BattlePlayer `json:"teams,omitempty"`

	// Players is used for solo Showdown. Nil for 3v3 and duo Showdown.
	Players []BattlePlayer `json:"players,omitempty"`
}

// BattlePlayer represents a participant within a battle.
type BattlePlayer struct {
	Tag     string        `json:"tag"`
	Name    string        `json:"name"`
	Brawler BattleBrawler `json:"brawler"`
}

// BattleBrawler is the brawler-in-use snapshot inside a battle record.
type BattleBrawler struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Power    int    `json:"power"`
	Trophies int    `json:"trophies"`
}

// ─── Brawlers ────────────────────────────────────────────────────────────────

// Brawler represents a brawler entry from GET /brawlers or GET /brawlers/{id}.
type Brawler struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	StarPowers []StarPower `json:"starPowers"`
	Gadgets    []Gadget    `json:"gadgets"`
}

// BrawlerList is the response shape for GET /brawlers.
type BrawlerList = ListResponse[Brawler]

// ─── Events / Rotation ───────────────────────────────────────────────────────

// RotationSlot represents one slot in the current event rotation.
type RotationSlot struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	SlotID    int    `json:"slotId"`
	Event     Event  `json:"event"`
}

// Event describes a game mode + map combination.
type Event struct {
	ID   int    `json:"id"`
	Mode string `json:"mode"`
	Map  string `json:"map"`
}

// EventRotation is the response for GET /events/rotation (a plain JSON array).
type EventRotation []RotationSlot

// ─── Rankings ────────────────────────────────────────────────────────────────

// RankedPlayer represents a player on a leaderboard.
type RankedPlayer struct {
	Tag       string          `json:"tag"`
	Name      string          `json:"name"`
	NameColor string          `json:"nameColor"`
	Icon      PlayerIcon      `json:"icon"`
	Trophies  int             `json:"trophies"`
	Rank      int             `json:"rank"`
	Club      *PlayerClubStub `json:"club,omitempty"`
}

// RankingList is the response for rankings endpoints.
type RankingList = ListResponse[RankedPlayer]

// ─── Clubs ───────────────────────────────────────────────────────────────────

// Club represents a Brawl Stars club returned by GET /clubs/{tag}.
type Club struct {
	Tag               string        `json:"tag"`
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Type              string        `json:"type"` // "open" | "inviteOnly" | "closed"
	BadgeID           int           `json:"badgeId"`
	RequiredTrophies  int           `json:"requiredTrophies"`
	Trophies          int           `json:"trophies"`
	Members           []ClubMember  `json:"members"`
}

// ClubMember represents a member inside a Club.
type ClubMember struct {
	Tag       string     `json:"tag"`
	Name      string     `json:"name"`
	Role      string     `json:"role"` // "member" | "senior" | "vicePresident" | "president"
	Trophies  int        `json:"trophies"`
	NameColor string     `json:"nameColor"`
	Icon      PlayerIcon `json:"icon"`
}

// ClubMemberList is the response for GET /clubs/{tag}/members.
type ClubMemberList = ListResponse[ClubMember]
