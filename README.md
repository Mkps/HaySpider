# Hay_Spider

A performance aware crawler/spider written in Golang helping you find a specific string or pattern inside a given website. Learning project / Open to code reviews.

## Why Go?

Go is a simple yet powerful language especially for applications geared toward web/cybersecurity. I was inspired by the performance of ffuf and so wanted to get working on this.

## The program

The program's purpose is to extract images inside a website starting from a root URL. It has 3 flags:
- "-r" to specify recursive extraction.
- "-l" to set the recursive level. If not set will default to 5.
- "-p" to specify output folder. If not set will default to ./data/
The program will stay confined to the root of the starting URL.
I want to write it in imperative programming instead of using recursion. That will lighten the burden on the stack and consequently allow it to go deeper than if using recursion.
Potential improvement: The fact that multiple address are targeted means that a big gain in performance is possible if it were to be parallelized.

## Implementation

In pseudo-code, what the program will do once the parsing is done is:
- Add the root URL to a stack. 
- Find all URLs within the page and add them to the queue channel if they are inside the website and are not inside the stack.
- Check if the content of the current page should be investigated further.
- If so, parse the content looking for target -> if target is found download the file
- Go to the next item in the stack.

## WIP
- Needs to add runtime option to set target
- Refactor to reduce argument count / set up a struct
- Create a log linking url addresses and files.
