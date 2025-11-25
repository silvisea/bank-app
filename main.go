package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Кастомные ошибки
var (
	ErrInsufficientFunds   = errors.New("недостаточно средств на счете")
	ErrInvalidAmount       = errors.New("некорректная сумма (отрицательная или нулевая)")
	ErrAccountNotFound     = errors.New("счет не найден")
	ErrSameAccountTransfer = errors.New("попытка перевода на тот же счёт")
)

// Интерфейсы
type AccountService interface {
	Deposit(amount float64) error
	Withdraw(amount float64) error
	Transfer(to *Account, amount float64) error
	GetBalance() float64
	GetStatement() string
}

type Storage interface {
	SaveAccount(account *Account) error
	LoadAccount(accountID string) (*Account, error)
	GetAllAccounts() ([]*Account, error)
}

// Domain модели
type TransactionType string

const (
	Deposit  TransactionType = "DEPOSIT"
	Withdraw TransactionType = "WITHDRAW"
	Transfer TransactionType = "TRANSFER"
)

type Transaction struct {
	ID          string
	Type        TransactionType
	Amount      float64
	Timestamp   time.Time
	Description string
}

type Account struct {
	ID           string
	OwnerName    string
	Balance      float64
	Transactions []Transaction
}

func NewAccount(ownerName string) *Account {
	return &Account{
		ID:           generateID(),
		OwnerName:    ownerName,
		Balance:      0.0,
		Transactions: make([]Transaction, 0),
	}
}

func generateID() string {
	return fmt.Sprintf("ACC%d", time.Now().UnixNano())
}

func (a *Account) AddTransaction(tType TransactionType, amount float64, description string) {
	transaction := Transaction{
		ID:          fmt.Sprintf("TX%d", time.Now().UnixNano()),
		Type:        tType,
		Amount:      amount,
		Timestamp:   time.Now(),
		Description: description,
	}
	a.Transactions = append(a.Transactions, transaction)
}

// Реализация Storage
type MemoryStorage struct {
	accounts map[string]*Account
	mutex    sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		accounts: make(map[string]*Account),
	}
}

func (s *MemoryStorage) SaveAccount(account *Account) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.accounts[account.ID] = account
	return nil
}

func (s *MemoryStorage) LoadAccount(accountID string) (*Account, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	account, exists := s.accounts[accountID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *MemoryStorage) GetAllAccounts() ([]*Account, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	accounts := make([]*Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// Реализация AccountService
type AccountServiceImpl struct {
	account *Account
	storage Storage
}

func NewAccountService(account *Account, storage Storage) AccountService {
	return &AccountServiceImpl{
		account: account,
		storage: storage,
	}
}

func (s *AccountServiceImpl) Deposit(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	s.account.Balance += amount
	s.account.AddTransaction(Deposit, amount, "Пополнение счета")

	return s.storage.SaveAccount(s.account)
}

func (s *AccountServiceImpl) Withdraw(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if s.account.Balance < amount {
		return ErrInsufficientFunds
	}

	s.account.Balance -= amount
	s.account.AddTransaction(Withdraw, amount, "Снятие средств")

	return s.storage.SaveAccount(s.account)
}

func (s *AccountServiceImpl) Transfer(to *Account, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if s.account.Balance < amount {
		return ErrInsufficientFunds
	}

	if s.account.ID == to.ID {
		return ErrSameAccountTransfer
	}

	// Снимаем с текущего счета
	s.account.Balance -= amount
	s.account.AddTransaction(Transfer, amount,
		fmt.Sprintf("Перевод на счет %s", to.ID))

	// Пополняем целевой счет
	to.Balance += amount
	to.AddTransaction(Transfer, amount,
		fmt.Sprintf("Перевод со счета %s", s.account.ID))
	// Сохраняем оба счета
	if err := s.storage.SaveAccount(s.account); err != nil {
		return err
	}
	return s.storage.SaveAccount(to)
}

func (s *AccountServiceImpl) GetBalance() float64 {
	return s.account.Balance
}

func (s *AccountServiceImpl) GetStatement() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Выписка по счету %s\n", s.account.ID))
	sb.WriteString(fmt.Sprintf("Владелец: %s\n", s.account.OwnerName))
	sb.WriteString(fmt.Sprintf("Текущий баланс: %.2f\n\n", s.account.Balance))
	sb.WriteString("История транзакций:\n")

	if len(s.account.Transactions) == 0 {
		sb.WriteString("Транзакций нет\n")
		return sb.String()
	}

	for _, tx := range s.account.Transactions {
		sb.WriteString(fmt.Sprintf("- %s: %.2f (%s) - %s\n",
			tx.Type, tx.Amount, tx.Timestamp.Format("02.01.2006 15:04:05"), tx.Description))
	}

	return sb.String()
}

// Основная логика приложения
func main() {
	storage := NewMemoryStorage()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("=== БАНКОВСКОЕ ПРИЛОЖЕНИЕ ===")

	for {
		showMainMenu()
		fmt.Print("Выберите действие: ")

		scanner.Scan()
		choice := scanner.Text()

		switch choice {
		case "1":
			createAccount(scanner, storage)
		case "2":
			selectAccount(scanner, storage)
		case "3":
			fmt.Println("Выход из программы...")
			return
		default:
			fmt.Println("Неверный выбор. Попробуйте снова.")
		}
	}
}

func showMainMenu() {
	fmt.Println("\n1. Создать счет")
	fmt.Println("2. Выбрать счет")
	fmt.Println("3. Выйти")
}

func createAccount(scanner *bufio.Scanner, storage *MemoryStorage) {
	fmt.Print("Введите имя владельца счета: ")
	scanner.Scan()
	ownerName := scanner.Text()

	if ownerName == "" {
		fmt.Println("Имя владельца не может быть пустым")
		return
	}

	account := NewAccount(ownerName)
	if err := storage.SaveAccount(account); err != nil {
		fmt.Printf("Ошибка создания счета: %v\n", err)
		return
	}

	fmt.Printf("Счет успешно создан! ID: %s\n", account.ID)
}

func selectAccount(scanner *bufio.Scanner, storage *MemoryStorage) {
	accounts, err := storage.GetAllAccounts()
	if err != nil {
		fmt.Printf("Ошибка получения счетов: %v\n", err)
		return
	}

	if len(accounts) == 0 {
		fmt.Println("Нет созданных счетов")
		return
	}

	fmt.Println("\nДоступные счета:")
	for i, acc := range accounts {
		fmt.Printf("%d. %s (Владелец: %s, Баланс: %.2f)\n",
			i+1, acc.ID, acc.OwnerName, acc.Balance)
	}

	fmt.Print("Выберите номер счета: ")
	scanner.Scan()
	choice, err := strconv.Atoi(scanner.Text())
	if err != nil || choice < 1 || choice > len(accounts) {
		fmt.Println("Неверный выбор")
		return
	}

	selectedAccount := accounts[choice-1]
	accountService := NewAccountService(selectedAccount, storage)
	runAccountMenu(scanner, accountService, storage, selectedAccount)
}

func runAccountMenu(scanner *bufio.Scanner, accountService AccountService, storage *MemoryStorage, account *Account) {
	for {
		showAccountMenu(account.OwnerName)
		fmt.Print("Выберите действие: ")

		scanner.Scan()
		choice := scanner.Text()

		switch choice {
		case "1":
			handleDeposit(scanner, accountService)
		case "2":
			handleWithdraw(scanner, accountService)
		case "3":
			handleTransfer(scanner, accountService, storage)
		case "4":
			handleShowBalance(accountService)
		case "5":
			handleGetStatement(accountService)
		case "6":
			return
		default:
			fmt.Println("Неверный выбор. Попробуйте снова.")
		}
	}
}

func showAccountMenu(ownerName string) {
	fmt.Printf("\n=== СЧЕТ: %s ===\n", ownerName)
	fmt.Println("1. Пополнить счет")
	fmt.Println("2. Снять средства")
	fmt.Println("3. Перевести другому счету")
	fmt.Println("4. Просмотреть баланс")
	fmt.Println("5. Получить выписку")
	fmt.Println("6. Вернуться в главное меню")
}

func handleDeposit(scanner *bufio.Scanner, accountService AccountService) {
	amount, err := getAmountFromUser(scanner, "Введите сумму для пополнения: ")
	if err != nil {
		return
	}

	if err := accountService.Deposit(amount); err != nil {
		fmt.Printf("Ошибка пополнения: %v\n", err)
	} else {
		fmt.Printf("Счет успешно пополнен на %.2f\n", amount)
	}
}
func handleWithdraw(scanner *bufio.Scanner, accountService AccountService) {
	amount, err := getAmountFromUser(scanner, "Введите сумму для снятия: ")
	if err != nil {
		return
	}

	if err := accountService.Withdraw(amount); err != nil {
		fmt.Printf("Ошибка снятия: %v\n", err)
	} else {
		fmt.Printf("Со счета успешно снято %.2f\n", amount)
	}
}

func handleTransfer(scanner *bufio.Scanner, accountService AccountService, storage *MemoryStorage) {
	amount, err := getAmountFromUser(scanner, "Введите сумму для перевода: ")
	if err != nil {
		return
	}

	fmt.Print("Введите ID счета получателя: ")
	scanner.Scan()
	toAccountID := scanner.Text()

	toAccount, err := storage.LoadAccount(toAccountID)
	if err != nil {
		fmt.Printf("Ошибка поиска счета: %v\n", err)
		return
	}

	if err := accountService.Transfer(toAccount, amount); err != nil {
		fmt.Printf("Ошибка перевода: %v\n", err)
	} else {
		fmt.Printf("Успешно переведено %.2f на счет %s\n", amount, toAccountID)
	}
}

func handleShowBalance(accountService AccountService) {
	balance := accountService.GetBalance()
	fmt.Printf("Текущий баланс: %.2f\n", balance)
}

func handleGetStatement(accountService AccountService) {
	statement := accountService.GetStatement()
	fmt.Println(statement)
}

func getAmountFromUser(scanner *bufio.Scanner, prompt string) (float64, error) {
	fmt.Print(prompt)
	scanner.Scan()
	amount, err := strconv.ParseFloat(scanner.Text(), 64)
	if err != nil {
		fmt.Println("Неверный формат суммы")
		return 0, err
	}
	return amount, nil
}
