# general identity

A third attempt to learn Chia lisp.

## Gotchas

- brun will not complain if a operator does not exist, maybe past a certain nr of characters
- short stings may be interpreted as number, or at least when printing
- not a clear overview if run also runs programs (or just compiles them), why is it called "run" then?
- It will not complain if an include doesn't exist
- sha256 build-in is differetn then the sha256tree lib, latter is for tree values

## Introduction

- The default meaning of a number is to access the node at the index of the "environment"
- The environment is what is passed as the second argument to `brun`
- The environment is structured as a binary tree, the nr 1 returns the whole tree
- The environment is passed in as an atom

### CLVM low level language

```
;;; Let's something foolish, let bron evaluate a integer:
brun '1' ; will not evaluate to '1' in Chialisp the numbers will eval to a position in the environment
brun '2' ; will complain, because th env is `()` and getting the second env position in that doesn't exist
brun '1' '400' ; will return 400, the "whole" environment is the 400
brun '1' '(400)' ; will return (400), the "whole" environment is a list with just 400
brun '1' '(400 500)' ; will return (400 500), the "whole" environment is a list with the two values
brun '1' '(1)' ; will NOT return '1', instead it will run brun '(1)' and attempt to run 1 as an operator, which is 'q'
brun '2' '(1)' ; will actually get the first argument in the env tree
brun '5' '(1 2)' ; will ACTUALLY get the second argument, given the UNBALANANCED binary env binary tree
brun '4' '((3 4) 2)' ; will return 3, given the left side of the binary env tree
;;; So what if we DO wanne print a literal integer
brun '(q . 2)' '(1)' ; we let brun run the 'q' operator that will allow us to mean the literal '2' number
;;; when brun receives a list, it will assume the first item is the operator
brun '(2)' ; will not print '2' or anything, it will try to turn 2 into an operator (apply), and complain for the lack of arguments
;;; Given that 16 equals to the 'apply' operator, what is th
brun '(16 2 2)' ; will NOT run (+ 2 2), remember that by default the nrs are positions in the env
brun '(16 2 2)' '(2)' ; will ACTUALLY run (+ 2 2) and output 4
brun '(16 (q . 2) (q . 2))' ; if we do not wanna pass in the literal through the env
brun '(+ (q . 2) (q . 2))' ; is a more readable way to do this ofcourse
;;; The 'q' operator it doesn't execute, but instead returns the right side as a value
brun '(q . 1)' ; outputs 1
brun '(q . (+ 1 2))' ; outputs (+ 1 2)
brun '(q . (1 2 3))' ; outputs (q 2 3), "1" is cast to its operator value of 'q'
;;; Printing output (for debugging)
brun '(x "hello, world")' ; again, path in atom error because the literal string here is evaluated to be env selection
brun '(x "b")' '((1 (((1 "hello")))) 2)' ; "b" is being cast to an int, causing a lookup into the env, until it runs into "hello" leaf
brun '(x (q . "hello, world"))' ; actually raises the message "hello, world"
brun '(x (q . "a"))' ; actually raises "97" probably because it is trying to cast "a" to an int
;;; conditions in clvm
brun '(if (= (f 1) (q . 100)) (q . "true") (q . "false"))' '(100)' ; does not work in brun, 'if' is not an operator. use --strict to make this fail explicitely
brun '(i (= (f 1) (q . 100)) (q . "true") (q . "false"))' '(101)' ; works as expected: false
brun '(i (> (q . 101) (q . 100)) (x (q . "true")) (x (q . "false")))' ; i believe this happens because both branches will always run (confirmed: https://github.com/Chia-Network/clvm_tools#known-operators)
brun '(i (> (q . 101) (q . 100)) (q . "true") (q . "false"))' ; this works as expected: true
brun '(i (= (f 1) (q . 100)) (q . "true") (q . "false"))' '(101)' ; works as expected: false
;;; using apply to evaluate
brun '(a (x (q . "hello, world")) ())' ; will eval the program when running using "apply"
brun '(a 1 ())' '(x (q . "hello, world"))' ; will run a program passed in the the environment
brun '(a 1 (q . "hello, world"))' '(x 1)' ; will run the env as a program, then that program uses the env passed in from the original program
```

### Chia Lisp higher level language

```
;;; Let's move up a later of abstraction: run instead of brun
run '1' ; outputs "1".  The compiler recognizes and auto-quotes literals. [https://github.com/Chia-Network/clvm_tools#auto-quoting-of-literals]
run '(1)' ; outputs (1)
run '(+ 1 2)' ; outputs (3) does run it, or it compiles to a static value?
run '(f @)' '("hello, world")' ; outputs "hello, world", "@" represents the environment
;;; lets try a module
run '(mod (P1 P2) (x P2))' ; outputs the program that compiles to (x 5)
brun "$(run '(mod () (x))')" ; outputs the program which will simply fail
;;; hashing

;;; destructing examples

;;; lets try some conditions
brun '(if () (x (q . "true")) (x (q . "false")))' ; will raise "false", since () evals to nil/false
brun '(if 1 (x (q . "true")) (x (q . "false")))' ; will alse raise "false", since (1) is false i guess, but "ever atom is considered to be true" (?)
brun '(if (= (q . "a") (q . "b"))
    (x (q . "true"))
    (x (q . "false"))
)' ; why is it not false? should use "run" here?, because it's not "bytecode" execute?

;;; modules evaluation
(mod ()
    (x (q . "foo")) ; ignored, not evaluated
    (x (q . "bar")) ; entry point
) ; will raise "bar", "foo" is ignored
```

### Actually locking funds and spending the coin

- Explain what it means to "send" xch to the address of our coin (commitment)
- Explain the steps to do so, compile to clvm, hash it, encode it
- Explain what it means to "unlock" the xch in our coin by providing a solution
- Explain why it is called the "reveal"

### Keybase resources

- A) hi all, sorry if its a silly question, but i just started learning the chialisp
  so when i run '(list 1 2 3)' why does it convert into (q 2 3) in the result/output? where did the first atom go? thanks
- B) It returned `(1 2 3)` but because everything is a program unless quoted, it _displayed_ first item as an operator. Operator code for `q` is 1
- A) so every first item in the list would always be converted as an operator. is this correct? like if i write (list 11 2 3) then it will convert it into (sha256 2 3)
- B) Correct
- B) it runs list operator, which returns (1 2 3) data structure. This can now be interpreted in different ways. We might output it or it might be run within another program and return (a 3). Essentially a program always returns a program, but it's up to the runner that decides in the end how it's interpreted. This is very powerful but can be confusing in the beginning.
