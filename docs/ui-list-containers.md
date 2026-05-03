# UI List Containers

Toda lista renderizada dentro de um container/painel deve usar formato compacto.

## Regra

- Cada item deve ocupar exatamente uma linha.
- A lista deve mostrar a maior quantidade possível de itens dentro da altura disponível.
- Deve existir um cursor visível para seleção, usando `>` no item ativo.
- Não usar descrição em segunda linha para itens de lista dentro de containers.
- Não mostrar título, paginação, status bar ou help internos da `bubbles/list` quando o container já tiver título e footer.
- O scroll/navegação deve acontecer dentro da própria lista com `up/down`.

## Implementação Padrão

Use um delegate compacto de uma linha para `bubbles/list`, como o `compactDelegate` em `internal/tui/list_delegate.go`.

Configuração esperada:

```go
items := list.New([]list.Item{}, compactDelegate{}, 0, 0)
items.SetShowHelp(false)
items.SetShowTitle(false)
items.SetShowPagination(false)
items.SetFilteringEnabled(false)
items.SetShowStatusBar(false)
```

O container externo é responsável pelo título visual, borda, altura e foco ativo.

## Exemplo Visual

```txt
┌──── Apps ────┐
│> com.foo.app │
│  chrome      │
│  settings    │
│  camera      │
│  maps        │
└──────────────┘
```

Evitar este formato:

```txt
┌──── Apps ────┐
│> com.foo.app │
│              │
│  chrome      │
│              │
└──────────────┘
```
