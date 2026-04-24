# 代码块测试 Demo

## 多语言代码块

### Go

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    http.ListenAndServe(":8080", nil)
}
```

### JavaScript

```javascript
const fs = require('fs');
const path = require('path');

function readDir(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    return entries.map(entry => ({
        name: entry.name,
        type: entry.isDirectory() ? 'dir' : 'file',
    }));
}

console.log(readDir('.'));
```

### Python

```python
import os
from pathlib import Path

def walk_directory(root: str):
    """递归遍历目录"""
    for dirpath, dirnames, filenames in os.walk(root):
        for name in filenames:
            filepath = Path(dirpath) / name
            print(f"{filepath} ({filepath.stat().st_size} bytes)")

if __name__ == "__main__":
    walk_directory(".")
```

### Rust

```rust
use std::fs;
use std::path::Path;

fn main() {
    let path = Path::new(".");
    for entry in fs::read_dir(path).unwrap() {
        let entry = entry.unwrap();
        println!("{:?}", entry.path());
    }
}
```

### TypeScript

```typescript
interface FileInfo {
    name: string;
    type: 'file' | 'dir';
    size?: number;
    modified?: string;
}

async function fetchFiles(path: string): Promise<FileInfo[]> {
    const response = await fetch(`/api/files?path=${encodeURIComponent(path)}`);
    return response.json();
}

fetchFiles('/docs').then(files => {
    console.log(files);
});
```

### SQL

```sql
SELECT
    u.name,
    COUNT(f.id) as file_count,
    MAX(f.modified) as last_modified
FROM users u
LEFT JOIN files f ON f.user_id = u.id
WHERE u.active = 1
GROUP BY u.id
ORDER BY file_count DESC
LIMIT 10;
```

### JSON

```json
{
    "name": "clawbench",
    "version": "1.0.0",
    "features": [
        "markdown-rendering",
        "mermaid-diagrams",
        "syntax-highlighting",
        "ai-chat"
    ],
    "config": {
        "port": 20000,
        "watch_dir": "/path/to/markdown",
        "password": null
    }
}
```

### YAML

```yaml
server:
  host: localhost
  port: 20000
  timeout: 30s

features:
  markdown: true
  mermaid: true
  highlight: true
  ai_chat: true

security:
  password_protection: false
  session_duration: 7d
```

### Shell

```bash
#!/bin/bash

# Build and run
go build -o clawbench .
./clawbench

# Or with custom config
WATCH_DIR=/path/to/markdown ./clawbench
```

### CSS

```css
:root {
    --color-bg: #ffffff;
    --color-text: #24292e;
    --color-accent: #0366d6;
}

.content {
    max-width: 900px;
    margin: 0 auto;
    padding: 2rem;
}

.code-block {
    background: var(--color-bg);
    border: 1px solid #e1e4e8;
    border-radius: 6px;
    overflow-x: auto;
}
```

### Java

```java
public class FileWatcher {
    private final Path directory;
    private final WatchService watcher;

    public FileWatcher(Path directory) throws IOException {
        this.directory = directory;
        this.watcher = FileSystems.getDefault().newWatchService();
    }

    public void watch() throws InterruptedException {
        WatchKey key;
        while ((key = watcher.take()) != null) {
            for (WatchEvent<?> event : key.pollEvents()) {
                System.out.println("Event: " + event.kind() + " - " + event.context());
            }
            key.reset();
        }
    }
}
```

## 行内代码

这是一段 `inline code` 示例，展示了行内代码的渲染效果。可以在段落中 `console.log('hello')` 这样使用。

## 无语言标识的代码块

```
这是一个没有指定语言的代码块。
应该使用默认的高亮样式渲染。
```
