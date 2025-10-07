# ft_spider

A performance aware crawler/image-extractor written in Golang. Learning project / Open to code reviews.

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
- Find all URLs within the page and add them to the stack if they are inside the website and are not inside the stack.
- Get all images from the current page matching the file extension.
- Go to the next item in the stack.
