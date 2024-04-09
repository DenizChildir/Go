package main

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/template/html/v2"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {

	//engine := html.New("./views", ".html")
	//app := fiber.New(fiber.Config{
	//	Views: engine,
	//})
	//app.Static("/", "./public")
	//app.Get("/", ListFacts)
	//app.Listen(":3000")

	contactRepo := NewContactJSONRepository()
	contactUseCase := contactRepo.(*contactJSONRepository)
	app := application{
		contactsUseCase: contactUseCase,
	}
	engine := html.New("./views", ".html")

	app2 := fiber.New(fiber.Config{
		Views: engine,
	})

	app2.Static("/static", "./static")

	app2.Get("/", app.contactsFiber)
	app2.Get("/contacts/count", app.count)
	app2.Get("/contacts/new", app.contactsNewGetFiber)
	app2.Post("/contacts/new", app.contactsNew)
	app2.Get("/contacts/:id", app.contactsView)
	app2.Get("/contacts/:id/edit", app.contactsEditGet)
	app2.Post("/contacts/:id/edit", app.contactsEditPost)
	app2.Get("contacts/:id/email", app.contactsEmailGet)
	app2.Delete("/contacts/:id", app.contactsDelete)
	app2.Delete("", app.contactsDeleteAll)

	app2.Listen(":3000")
}

func ListFacts(c *fiber.Ctx) error {
	return c.Render("index", fiber.Map{
		"Title":    "Div Rhino Trivia Time",
		"Subtitle": "Facts for funtimes with friends!",
	})
}
func (app *application) contactsFiber(c *fiber.Ctx) error {
	var contacts []Contact
	search := c.Query("q")
	if search != "" {
		contacts, _ = app.contactsUseCase.Search(search)
	} else {
		contacts, _ = app.contactsUseCase.List()
	}
	if c.Get("HX-Trigger") == "search" {
		return c.Render("index", fiber.Map{
			"contacts": contacts,
			"search":   search})
	}
	fl, _ := flashes(c)
	return c.Render("index", fiber.Map{
		"contacts": contacts,
		"messages": fl})

}
func (app *application) count(c *fiber.Ctx) error {
	count := app.contactsUseCase.Count()
	return c.SendString(fmt.Sprintf("(%d total Contacts)", count))
}
func (app *application) contactsNewGetFiber(c *fiber.Ctx) error {
	fmt.Println("I ran")
	fl, _ := flashes(c)
	contact := Contact{}
	return c.Render("new", fiber.Map{
		"contact":  contact,
		"messages": fl})
}

func (app *application) contactsNew(c *fiber.Ctx) error {
	var f newContactForm
	if err := c.BodyParser(&f); err != nil {
		return c.Render("new.html", fiber.Map{
			"contact": Contact{
				First: f.FirstName,
				Last:  f.LastName,
				Phone: f.Phone,
				Email: f.Email,
			},
			"errors": errorList{Email: err.Error()},
		})
	}

	err := app.contactsUseCase.CreateStr(f.FirstName, f.LastName, f.Phone, f.Email)
	if err != nil {
		return c.Render("new.html", fiber.Map{
			"contact": Contact{
				First: f.FirstName,
				Last:  f.LastName,
				Phone: f.Phone,
				Email: f.Email,
			},
			"errors": errorList{Email: err.Error()},
		})
	}

	flashMessage(c, "Created New Contact!")
	return c.Redirect("/contacts")
}

func (app *application) contactsView(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).SendString(err.Error())
	}

	contact, err := app.contactsUseCase.Find(id)
	fmt.Println(contact)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	messages, _ := flashes(c)
	return c.Render("show", fiber.Map{
		"contact":  contact,
		"messages": messages,
	})
}
func (app *application) contactsEditGet(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).SendString(err.Error())
	}

	contact, err := app.contactsUseCase.Find(id)
	fmt.Println(contact)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	messages, _ := flashes(c)
	return c.Render("edit", fiber.Map{
		"contact":  contact,
		"messages": messages,
	})
}

func (app *application) contactsEditPost(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	var f newContactForm
	if err := c.BodyParser(&f); err != nil {
		return err
	}

	contact := Contact{
		ID:    id,
		First: f.FirstName,
		Last:  f.LastName,
		Phone: f.Phone,
		Email: f.Email,
	}

	if err := app.contactsUseCase.Update(contact); err != nil {
		errs := errorList{Email: err.Error()} // currently only email errors given, otherwise we could check errors.Is?
		return c.Render("edit.html", fiber.Map{"contact": contact, "errors": errs})
	}

	flashMessage(c, "Updated Contact!")
	return c.Redirect(fmt.Sprintf("/contacts/%d", id))
}

func (app *application) contactsEmailGet(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	email := c.Query("email")

	contact, _ := app.contactsUseCase.Find(id)
	contact.Email = email

	if err := app.contactsUseCase.Validate(*contact); err != nil {
		return c.SendString(err.Error())
	}

	return c.SendString("")
}

func (app *application) contactsDelete(c *fiber.Ctx) error {
	fmt.Println("i ran")
	id, _ := strconv.Atoi(c.Params("id"))
	app.contactsUseCase.Delete(id)

	if c.Get("HX-Trigger") == "delete-btn" {
		flashMessage(c, "Deleted Contact!")
		return c.Redirect("/contacts")
	}

	return c.SendString("")
}

func (app *application) contactsDeleteAll(c *fiber.Ctx) error {
	body := c.Request().Body()
	q, _ := url.ParseQuery(string(body))
	contactIDs := q["selected_contact_ids"]

	for _, cid := range contactIDs {
		id, _ := strconv.Atoi(cid)
		app.contactsUseCase.Delete(id)
	}

	flashMessage(c, "Deleted Contacts!")
	contacts, _ := app.contactsUseCase.List()
	fl, _ := flashes(c)
	return c.Render("index.html", fiber.Map{"contacts": contacts, "messages": fl})
}

type application struct {
	contactsUseCase *contactJSONRepository
}

type contactJSONRepository struct {
	contactCache *Cache[Contact, int]
}

func flashMessage(c *fiber.Ctx, message string) error {
	store := session.New()
	session, err := store.Get(c)
	if err != nil {
		return err
	}

	session.Set("flash", message)
	if err := session.Save(); err != nil {
		return fmt.Errorf("error in flashMessage saving session: %s", err)
	}

	return nil
}

func flashes(c *fiber.Ctx) ([]interface{}, error) {
	store := session.New()
	session, err := store.Get(c)
	if err != nil {
		return nil, err
	}

	flash := session.Get("flash")
	if flash != nil {
		session.Delete("flash")
		if err := session.Save(); err != nil {
			return nil, fmt.Errorf("error in flashes saving session: %s", err)
		}
		return []interface{}{flash}, nil
	}

	return nil, nil
}

type newContactForm struct {
	FirstName string `form:"first_name"`
	LastName  string `form:"last_name"`
	Phone     string `form:"phone"`
	Email     string `form:"email"`
}

type errorList struct {
	First string
	Last  string
	Phone string
	Email string
}

type Cache[T any, U comparable] struct {
	lastUpdated time.Time
	entries     []T
	entriesMap  map[U]T
	mu          sync.RWMutex
}

// NewCache constructs a new cache containing a list of entries of the provided type
func NewCache[T any, U comparable](entries []T, entriesMap map[U]T) *Cache[T, U] {
	return &Cache[T, U]{
		entries:     entries,
		entriesMap:  entriesMap,
		lastUpdated: time.Now(),
	}
}

func (c *Cache[T, U]) LastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

func (c *Cache[T, U]) Update(entries []T, entriesMap map[U]T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = entries
	c.entriesMap = entriesMap
	c.lastUpdated = time.Now()
}

func (c *Cache[T, U]) Retrieve() []T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries
}

func (c *Cache[T, U]) RetrieveMap() map[U]T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entriesMap
}

func (c *Cache[T, U]) RetrieveMapEntry(id U) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entriesMap[id]
	return entry, ok
}

type Contact struct {
	ID    int
	First string
	Last  string
	Phone string
	Email string
}

type Err string

func (e Err) Error() string {
	return string(e)
}

const (
	// Validation Errors
	ErrMissingEmail    = Err("email required")
	ErrExistingContact = Err("email must be unique")
)

func LoadJSON(filename string, v interface{}) error {
	file, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(file), &v)
	return err
}

func SaveJSON(filename string, data interface{}) error {
	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, file, 0644)
}

type ContactUseCase interface {
	Create(first, last, phone, email string) error
	List() ([]Contact, error)
	Update(Contact) error
	Delete(id int) error
	Search(query string) ([]Contact, error)
	Find(id int) (*Contact, error)
	Count() int
	Validate(Contact) error
}

type ContactRepository interface {
	Create(contact Contact) error
	List() ([]Contact, error)
	Update(Contact) error
	Delete(id int) error
	Search(query string) ([]Contact, error)
	Find(id int) (*Contact, error)
	Count() int
	Validate(Contact) error
}

func NewContactJSONRepository() ContactRepository {
	var contacts []Contact
	err := LoadJSON("contacts.json", &contacts)
	if err != nil {
		log.Fatal(err)
	}
	contactsMap := make(map[int]Contact)
	for _, c := range contacts {
		contactsMap[c.ID] = c
	}
	contactCache := NewCache(contacts, contactsMap)
	return &contactJSONRepository{
		contactCache: contactCache,
	}
}

func (r *contactJSONRepository) Create(contact Contact) error {
	err := r.Validate(contact)
	if err != nil {
		return err
	}
	var ids []int
	contacts := r.contactCache.Retrieve()
	for _, c := range contacts {
		ids = append(ids, c.ID)
	}
	contactsMap := r.contactCache.RetrieveMap()
	maxid := 0
	if len(contacts) != 0 {
		max := slices.Max(ids)
		fmt.Println("MAX:", max)
		maxid = max + 1

	}
	contact.ID = maxid
	contacts = append(contacts, contact)
	contactsMap[maxid] = contact
	r.contactCache.Update(contacts, contactsMap)
	SaveJSON("contacts.json", contacts)
	return nil
}

func (r *contactJSONRepository) CreateStr(FirstName, LastName, Phone, Email string) error {
	contact := Contact{First: FirstName, Last: LastName, Phone: Phone, Email: Email}
	err := r.Validate(contact)
	if err != nil {
		return err
	}
	var ids []int
	contacts := r.contactCache.Retrieve()
	for _, c := range contacts {
		ids = append(ids, c.ID)
	}
	contactsMap := r.contactCache.RetrieveMap()
	maxid := 0
	if len(contacts) != 0 {
		max := slices.Max(ids)
		fmt.Println("MAX:", max)
		maxid = max + 1

	}
	contact.ID = maxid
	contacts = append(contacts, contact)
	contactsMap[maxid] = contact
	r.contactCache.Update(contacts, contactsMap)
	SaveJSON("contacts.json", contacts)
	return nil
}

func (r *contactJSONRepository) List() ([]Contact, error) {
	return r.contactCache.Retrieve(), nil
}

func (r *contactJSONRepository) Update(contact Contact) error {
	err := r.Validate(contact)
	if err != nil {
		return err
	}
	contacts := r.contactCache.Retrieve()
	var contactPos int
	for i, c := range contacts {
		if c.ID == contact.ID {
			contactPos = i
		}
	}
	contacts[contactPos] = contact
	contactsMap := r.contactCache.RetrieveMap()
	contactsMap[contact.ID] = contact
	r.contactCache.Update(contacts, contactsMap)
	SaveJSON("contacts.json", contacts)
	return nil
}

func (r *contactJSONRepository) Delete(id int) error {
	contactsMap := r.contactCache.RetrieveMap()
	delete(contactsMap, id)
	contacts := r.contactCache.Retrieve()
	var contactPos int
	for i, c := range contacts {
		if c.ID == id {
			contactPos = i
			break
		}
	}
	newContacts := append(contacts[:contactPos], contacts[contactPos+1:]...)
	r.contactCache.Update(newContacts, contactsMap)
	SaveJSON("contacts.json", newContacts)
	return nil
}

func (r *contactJSONRepository) Search(query string) ([]Contact, error) {
	var results []Contact
	contacts := r.contactCache.Retrieve()
	for _, c := range contacts {
		matchFirst := strings.Contains(strings.ToLower(c.First), strings.ToLower(query))
		matchLast := strings.Contains(strings.ToLower(c.Last), strings.ToLower(query))
		matchEmail := strings.Contains(strings.ToLower(c.Email), strings.ToLower(query))
		matchPhone := strings.Contains(strings.ToLower(c.Phone), strings.ToLower(query))
		if matchFirst || matchLast || matchEmail || matchPhone {
			results = append(results, c)
		}
	}
	return results, nil
}

func (r *contactJSONRepository) Find(id int) (*Contact, error) {
	contact, _ := r.contactCache.RetrieveMapEntry(id)
	return &contact, nil
}

func (r *contactJSONRepository) Count() int {
	return len(r.contactCache.Retrieve())
}

func (r *contactJSONRepository) Validate(contact Contact) error {
	if contact.Email == "" {
		return ErrMissingEmail
	}
	contacts := r.contactCache.Retrieve()
	for _, c := range contacts {
		if strings.EqualFold(c.Email, contact.Email) && c.ID != contact.ID {
			return ErrExistingContact
		}
	}
	return nil
}

type contactUseCase struct {
	contactRepo ContactRepository
}

func NewContactUseCase(contactRepo ContactRepository) ContactUseCase {
	return &contactUseCase{
		contactRepo: contactRepo,
	}
}

func (c *contactUseCase) Create(first, last, phone, email string) error {
	return c.contactRepo.Create(Contact{
		First: first,
		Last:  last,
		Phone: phone,
		Email: email,
	})
}

func (c *contactUseCase) List() ([]Contact, error) {
	return c.contactRepo.List()
}

func (c *contactUseCase) Update(contact Contact) error {
	return c.contactRepo.Update(contact)
}

func (c *contactUseCase) Delete(id int) error {
	return c.contactRepo.Delete(id)
}

func (c *contactUseCase) Search(query string) ([]Contact, error) {
	return c.contactRepo.Search(query)
}

func (c *contactUseCase) Find(id int) (*Contact, error) {
	return c.contactRepo.Find(id)
}

func (c *contactUseCase) Count() int {
	time.Sleep(2 * time.Second)
	return c.contactRepo.Count()
}

func (c *contactUseCase) Validate(contact Contact) error {
	return c.contactRepo.Validate(contact)
}
