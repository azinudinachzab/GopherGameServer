// This package contains all the necessary tools to make and work with Rooms. A Room represents
// a place on the server where a User can join others Users. Rooms also have their own variables which
// can be accessed and changed anytime. A Room variable can be anything compatible with interface{}, so
// pretty much anything.
package rooms

import (
	"github.com/hewiefreeman/GopherGameServer/helpers"
	"github.com/gorilla/websocket"
	"errors"
)

//
type Room struct {
	name string
	rType string

	private bool
	owner string;
	inviteList *[]string

	usersMap *map[string]RoomUser
	maxUsers int

	vars *map[string]interface{}

	roomVarsActionChannel *helpers.ActionChannel
	usersActionChannel *helpers.ActionChannel
}

// A representation of a User in a Room. These store a User's variables. Note: These
// are not the Users themselves. If you need to get a User type from one of these, use
// users.RoomUser() to convert a RoomUser into a User.
type RoomUser struct {
	name *string

	vars map[string]interface{}

	socket *websocket.Conn
}

var (
	rooms map[string]*Room = make(map[string]*Room)
	roomsActionChan *helpers.ActionChannel = helpers.NewActionChannel()
	serverStarted bool = false
)

//SEVER START-UP FUNCTIONS
func SetServerStarted(val bool){
	if(!serverStarted){
		serverStarted = val;
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   MAKE A NEW ROOM   ////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Adds a new room to the server. This can be called before or after starting the server.
// Parameters:
//  - name (string): Name of the Room
//  - rType (string): Room type name
//  - isPrivate (bool): Indicates if the room is private or not
//  - maxUsers (int): Maximum User capacity
func New(name string, rType string, isPrivate bool, maxUsers int, owner string) (Room, error) {
	//REJECT INCORRECT INPUT
	if(len(name) == 0){ return Room{}, errors.New("rooms.New() requires a name"); }

	var err error = nil;

	response := roomsActionChan.Execute(newRoom, []interface{}{name, maxUsers, isPrivate, rType, owner});
	if(response[1] != nil){ err = response[1].(error); }

	return response[0].(Room), err;
}

func newRoom(p []interface{}) []interface{} {
	roomName, maxUsers, isPrivate, rt, owner := p[0].(string), p[1].(int), p[2].(bool), p[3].(string), p[4].(string);
	var room Room = Room{};
	var err error = nil;

	if _, ok := rooms[roomName]; ok {
		err = errors.New("A Room with the name '"+roomName+"' already exists");
	}else{
		userMap := make(map[string]RoomUser);
		roomVars := make(map[string]interface{});
		roomVarsActionChan := helpers.NewActionChannel();
		roomUsersActionChan := helpers.NewActionChannel();
		invList := []string{};
		theRoom := Room{name: roomName, private: isPrivate, inviteList: &invList, usersMap: &userMap, maxUsers: maxUsers, vars: &roomVars,
					 owner: owner, rType: rt, roomVarsActionChannel: roomVarsActionChan, usersActionChannel: roomUsersActionChan};
		rooms[roomName] = &theRoom;
		room = *rooms[roomName];
	}
	//
	return []interface{}{room, err};
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   GET A ROOM   /////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Gets a Room. If the room does not exit, an error will be returned.
func Get(roomName string) (Room, error) {
	//REJECT INCORRECT INPUT
	if(len(roomName) == 0){ return Room{}, errors.New("rooms.Get() requires a room name"); }

	var err error = nil;

	response := roomsActionChan.Execute(getRoom, []interface{}{roomName});
	if(response[1] != nil){ err = response[1].(error) }

	//
	return response[0].(Room), err;
}

func getRoom(p []interface{}) []interface{} {
	roomName := p[0].(string);
	var err error = nil;
	var room Room = Room{}

	if _, ok := rooms[roomName]; ok {
		room = *rooms[roomName];
	}else{
		err = errors.New("The room '"+roomName+"' does not exist");
	}

	//
	return []interface{}{room, err};
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   ADD A USER   /////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// WARNING: This is only meant for internal Gopher Game Server mechanics. If you want a User to join a Room, use
// *User.Join() instead. Using this will break some server mechanics and potentially cause memory leaks.
func (r *Room) AddUser(userName *string, socket *websocket.Conn) error {
	//REJECT INCORRECT INPUT
	if(len(*userName) == 0){
		return errors.New("*Room.AddUser() requires a user name")
	}else if(socket == nil){
		return errors.New("*Room.AddUser() requires a user socket")
	}

	response := r.usersActionChannel.Execute(userJoin, []interface{}{userName, socket, r});
	if(len(response) == 0){
		return errors.New("The room '"+r.name+"' does not exist")
	}else if(response[0] != nil){
		return response[0].(error);
	}

	//
	return nil;
}

func userJoin(p []interface{}) []interface{} {
	userName, socket, room := p[0].(*string), p[1].(*websocket.Conn), p[2].(*Room);
	var err error = nil;

	//CHECK IF THE ROOM IS FULL
	if(len(*room.usersMap) == room.maxUsers){ err = errors.New("The room '"+room.name+"' is full"); }

	//
	if _, ok := (*room.usersMap)[*userName]; ok {
		err = errors.New("User '"+*userName+"' is already in room '"+room.name+"'");
	}else{
		(*room.usersMap)[*userName] = RoomUser{name: userName, socket: socket, vars: make(map[string]interface{})}
	}

	//
	return []interface{}{err}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   REMOVE A USER   //////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// WARNING: This is only meant for internal Gopher Game Server mechanics. If you want a User to leave a Room, use
// *User.Leave() instead. Using this will break some server mechanics and potentially cause memory leaks.
func (r *Room) RemoveUser(userName string) error {
	//REJECT INCORRECT INPUT
	if(len(userName) == 0){ return errors.New("*Room.RemoveUser() requires a user name") }

	response := r.usersActionChannel.Execute(userLeave, []interface{}{userName, r});
	if(len(response) == 0){
		return errors.New("The room '"+r.name+"' does not exist");
	}else if(response[0] != nil){
		return response[0].(error);
	}

	//
	return nil;
}

func userLeave(p []interface{}) []interface{} {
	userName, room := p[0].(string), p[1].(*Room);
	var err error = nil;

	if _, ok := (*room.usersMap)[userName]; ok {
		delete(*room.usersMap, userName);
	}else{
		err = errors.New("User '"+userName+"' is not in room '"+room.name+"'");
	}

	//
	return []interface{}{err}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   ADD TO inviteList   //////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// WARNING: This is only meant for internal Gopher Game Server mechanics. If you want to invite a User to a private room,
// use the *User.Invite() function instead.
//
// NOTE: You can still use this function safely, but remember that private rooms are designed to have an "owner",
// and only the owner should be able to send an invite and revoke an invitation.
func (r *Room) AddInvite(userName string) error {
	if(!r.private){
		return errors.New("Room is not private");
	}else if(len(userName) == 0){
		return errors.New("*Room.AddInvite() requires a userName");
	}

	response := r.roomVarsActionChannel.Execute(inviteUser, []interface{}{userName, r});
	if(len(response) == 0){
		return errors.New("The room '"+r.name+"' does not exist");
	}else if(response[0] != nil){
		return response[0].(error);
	}

	//
	return nil;
}

func inviteUser(p []interface{}) []interface{} {
	userName, room := p[0].(string), p[1].(*Room);

	theList := *(*room).inviteList;
	for i := 0; i < len(theList); i++ {
		if(theList[i] == userName){
			return []interface{}{errors.New("User '"+userName+"' is already on the invite list")}
		}
	}
	*(*room).inviteList = append(*(*room).inviteList, userName);
	//
	return []interface{}{nil}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   REMOVE FROM inviteList   /////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// WARNING: This is only meant for internal Gopher Game Server mechanics. If you want to remove a User from the room's
// private invite list, use the *User.RevokeInvite() function instead.
//
// NOTE: You can still use this function safely, but remember that private rooms are designed to have an "owner",
// and only the owner should be able to send an invite and revoke an invitation.
func (r *Room) RemoveInvite(userName string) error {
	if(!r.private){
		return errors.New("Room is not private");
	}else if(len(userName) == 0){
		return errors.New("*Room.RemoveInvite() requires a userName");
	}

	response := r.roomVarsActionChannel.Execute(uninviteUser, []interface{}{userName, r});
	if(len(response) == 0){
		return errors.New("The room '"+r.name+"' does not exist");
	}else if(response[0] != nil){
		return response[0].(error);
	}

	//
	return nil;
}

func uninviteUser(p []interface{}) []interface{} {
	userName, room := p[0].(string), p[1].(*Room);
	theList := *(*room).inviteList;
	for i := 0; i < len(theList); i++ {
		if(theList[i] == userName){
			*(*room).inviteList = append((*(*room).inviteList)[:i], (*(*room).inviteList)[i+1:]...)
			break;
		}
		if(i == len(theList)-1){
			return errors.New("User '"+userName+"' is not on the invite list");
		}
	}
	//
	return []interface{}{nil}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   GET A ROOM's inviteList   ////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Get a private Room's invite list
func (r *Room) InviteList() ([]string, error) {
	response := r.roomVarsActionChannel.Execute(getInviteList, []interface{}{r});
	if(len(response) == 0){
		return nil, errors.New("The room '"+r.name+"' does not exist");
	}

	//
	return response[0].([]string), nil;
}

func getInviteList(p []interface{}) []interface{} {
	room := p[0].(*Room);
	return []interface{}{*(*room).inviteList};
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   GET A Room's usersMap   //////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Retrieves a Map of all the RoomUsers.
func (r *Room) GetUserMap() (map[string]RoomUser, error) {
	var err error = nil;
	var userMap map[string]RoomUser = nil;

	response := r.usersActionChannel.Execute(userMapGet, []interface{}{r});
	if(len(response) == 0){
		err = errors.New("Room '"+r.name+"' does not exist");
	}else{
		userMap = response[0].(map[string]RoomUser);
	}

	return userMap, err;
}

func userMapGet(p []interface{}) []interface{} {
	room := p[0].(*Room);

	return []interface{}{*room.usersMap};
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   Room ATTRIBUTE READERS   /////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Gets the name of the Room.
func (r *Room) Name() string {
	return r.name;
}

// Gets the type of the Room.
func (r *Room) Type() string {
	return r.rType;
}

// Gets the type of the Room.
func (r *Room) IsPrivate() bool {
	return r.private;
}

// Gets the owner of the room
func (r *Room) Owner() bool {
	return r.owner;
}

// Gets the maximum User capacity of the Room.
func (r *Room) MaxUsers() int {
	return r.maxUsers;
}

// Gets the number of Users in the Room.
func (r *Room) NumUsers() int {
	m, e := r.GetUserMap();
	if(e != nil){ return 0; }
	return len(m);
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   RoomUser ATTRIBUTE READERS   /////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Gets the name of the RoomUser.
func (u *RoomUser) Name() string {
	return *(u.name);
}

// Gets a Map of the RoomUser's variables.
func (u *RoomUser) Vars() map[string]interface{} {
	return u.vars;
}