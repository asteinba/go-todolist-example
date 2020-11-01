package main

import (
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// Go's entrance function. Always named "main" in the "main" package.
func main() {
	// Create a instance of our ToDo controller to pass the different functions to the Gin router as seen a few lines below.
	th := NewTodoHandler(0)

	// Gin is our web api framework
	r := gin.Default()

	// Register our routes
	r.GET("/api/TodoItems", th.GetItems)
	r.GET("/api/TodoItems/:id", th.GetItemByID)
	r.POST("/api/TodoItems", th.PostItem)
	r.PUT("/api/TodoItems/:id", th.PutItem)
	r.DELETE("/api/TodoItems/:id", th.DeleteItem)

	// listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	r.Run()
}

// Our TodoItem
type TodoItem struct {
	Id         int
	Name       string
	IsComplete bool
}

// Create a custom TodoItem array (slice) with the three functions below type to make it sortable by id. One downside of Go: It has not generics, yet :(.
type TodoItemCollection []TodoItem

func (t TodoItemCollection) Len() int           { return len(t) }
func (t TodoItemCollection) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t TodoItemCollection) Less(i, j int) bool { return t[i].Id < t[j].Id }

// Same as our TodoItem but without the id and isComplete because a new item doesn't have a id and is never directly completed.
type PostTodoItem struct {
	Name string
}

// Same as our TodoItem but without the id because we cannot change the id of a item.
type PutTodoItem struct {
	Name       string
	IsComplete bool
}

// Go has no classic constructors you create instances of structs by normal functions.
func NewTodoHandler(lastID int) TodoHandler {
	return TodoHandler{
		items:  map[int]TodoItem{},
		lastID: lastID,
	}
}

// This struct is like a class MVC controller. It also holds the state of the Todo items.
// Instead of a real in memory database we just use a simple map paired with a read/write mutex for synchronization.
// Go has no classic classes it has structs with fields and you can add method to these struct as seen below for the GetItems function.
type TodoHandler struct {
	items  map[int]TodoItem
	lastID int
	sync.RWMutex
}

// The variable c of type gin.Context handels all the http stuff for us. It contains all methods we need for getting data from the request
// and out to the response. As seen below the JSON method writes out our map as JSON combined with a status code.
func (th *TodoHandler) GetItems(c *gin.Context) {
	// Here we are just read locking the map to prevent data races.
	th.RLock()
	// Lets convert our map into a array (slice in golang) just the be the same as the .NET Core application API.
	// We use a preallocated slice with the same capacity as the map to improve performance
	items := make(TodoItemCollection, len(th.items))
	i := 0
	for _, item := range th.items {
		items[i] = item
		i++
	}
	sort.Sort(items)
	c.JSON(http.StatusOK, items)
	th.RUnlock()
}

func (th *TodoHandler) GetItemByID(c *gin.Context) {
	// As url parameters are strings we first need to convert the string into a int
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Bad request: Id in path is not a valid id")
		return
	}

	// Read locking to prevent data races
	th.RLock()
	item, ok := th.items[id]
	if !ok {
		c.String(http.StatusNotFound, `Not found: Item with id "%v"`, id)
	} else {
		c.JSON(http.StatusOK, item)
	}
	th.RUnlock()
}

func (th *TodoHandler) PostItem(c *gin.Context) {
	// Create a instance of our PostTodoItem because we need to pass a pointer of it to ShouldBindJSON.
	// Gin will then deserialize the JSON for us into this struct.
	item := PostTodoItem{}
	// Deserialize the JSON body into our item
	err := c.ShouldBindJSON(&item)
	if err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	// Write locking cause we are going to write into the TodoHandler
	th.Lock()
	// Increment the id counter to fake real database id's.
	th.lastID++
	// Assign the
	th.items[th.lastID] = TodoItem{
		Id:         th.lastID,
		Name:       item.Name,
		IsComplete: false,
	}
	th.Unlock()
}

func (th *TodoHandler) PutItem(c *gin.Context) {
	putItem := PutTodoItem{}
	err := c.ShouldBindJSON(&putItem)
	if err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Bad request: Id in url is not a valid id")
		return
	}

	th.Lock()
	// Defer calls the statement behind after the function has returned. We use defer here to make sure we unlock the map again.
	// We can forget it inside of the early return, so its better to use defer to make sure we unlock it to prevent a deadlock.
	defer th.Unlock()
	item, ok := th.items[id]
	if !ok {
		c.String(http.StatusNotFound, `Not found: Item with id "%v"`, id)
		return
	}
	// Tricky one liner :P. We assign Name and IsComplete from the put Item and after this we directly assign the modified item back to
	// the map because Go supports multiple assignments in one line using commas.
	item.Name, item.IsComplete, th.items[id] = putItem.Name, putItem.IsComplete, item
}

func (th *TodoHandler) DeleteItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Bad request: Id in url is not a valid id")
		return
	}

	th.Lock()
	// Just check if the item exist in the map. The underscore is used to ignore the returned item, to safe memory.
	_, ok := th.items[id]
	if !ok {
		c.String(http.StatusNotFound, `Not found: Item with id "%v"`, id)
		return
	}
	// Delete the item from the map
	delete(th.items, id)
	th.Unlock()
}
