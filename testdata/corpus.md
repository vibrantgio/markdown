# Markdown corpus

This corpus exercises every construct the G6.2 block renderer supports: it
stands in for the 2026-07-20 `gioui.org/x/markdown` evaluation document.

Inline styles: **bold**, *italic*, ***bold italic***, `inline code`,
a [prism link](https://github.com/vibrantgio/prism), ~~struck text~~,
and an autolink https://gioui.org for good measure.

## Lists

- first bullet
- second bullet holds **bold** text
  - nested bullet
    - deeper still
- third bullet

1. step one
2. step two
   1. sub-step

Ordered lists can start anywhere:

7. seventh
8. eighth

### Tasks

- [x] shipped task
- [ ] open task

#### Quotes

> Quoted paragraph with *emphasis* inside.
>
> > A nested quote one level deeper.

##### Code

```go
func main() {
	if true {
		fmt.Println("tabs expand to spaces")
	}
}
```

    indented code block
    second line

###### Break below

---

Soft
break becomes a space, and a hard\
break becomes a new line.
