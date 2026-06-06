# Sending Buttons and Lists via whatsmeow

> Implementation reference for building WhatsApp interactive messages (buttons and lists) using
> the whatsmeow Go library. This document is API-agnostic — it covers only the **payload construction**
> and **whatsmeow calls** so you can replicate the same behavior in any Go service.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Imports](#imports)
- [Core Concepts](#core-concepts)
- [Buttons Message](#buttons-message)
  - [Input Parameters](#buttons-input-parameters)
  - [Protobuf Construction](#buttons-protobuf-construction)
  - [Extra Binary Nodes](#buttons-extra-binary-nodes)
  - [Complete Buttons Code](#complete-buttons-code)
- [List Message](#list-message)
  - [Input Parameters](#list-input-parameters)
  - [Protobuf Construction](#list-protobuf-construction)
  - [Extra Binary Nodes](#list-extra-binary-nodes)
  - [Complete List Code](#complete-list-code)
- [Sending the Message](#sending-the-message)
- [JID Parsing](#jid-parsing)
- [Validation Rules](#validation-rules)
- [Common Pitfalls](#common-pitfalls)
- [Testing](#testing)
- [Full Working Example](#full-working-example)

---

## Prerequisites

```bash
go get go.mau.fi/whatsmeow@latest
go get google.golang.org/protobuf
```

You need a connected `*whatsmeow.Client` instance. This document assumes you already have one.

---

## Imports

```go
import (
    "context"
    "strings"

    "go.mau.fi/whatsmeow"
    waBinary "go.mau.fi/whatsmeow/binary"
    waProto  "go.mau.fi/whatsmeow/proto/waE2E"
    waTypes  "go.mau.fi/whatsmeow/types"
    "google.golang.org/protobuf/proto"
)
```

---

## Core Concepts

### 1. FutureProofMessage Wrapper

WhatsApp interactive messages (buttons, lists) **must** be wrapped inside a `DocumentWithCaptionMessage` containing a `FutureProofMessage`. This is the key trick that makes them actually render on the recipient's device:

```go
// The interactive message (buttons or list) goes INSIDE this wrapper
finalMessage := &waProto.Message{
    DocumentWithCaptionMessage: &waProto.FutureProofMessage{
        Message: &waProto.Message{
            ButtonsMessage: buttonsMsg,  // or ListMessage: listMsg
        },
    },
}
```

> **Why?** WhatsApp Web / Desktop uses this wrapper to handle forward-compatibility for message
> types that older clients may not understand. Without it, the message arrives as plain text or
> is silently dropped.

### 2. Additional Binary Nodes

Both buttons and lists require **extra binary XML nodes** attached to the stanza. These nodes are
injected via `whatsmeow.SendRequestExtra.AdditionalNodes` and signal to the WhatsApp server how
to process the interactive message.

### 3. SendMessage with Extra

```go
// Standard send (text, media, etc.)
resp, err := client.SendMessage(ctx, chatJID, message)

// Interactive send (buttons, lists) — needs extra nodes
resp, err := client.SendMessage(ctx, chatJID, message, whatsmeow.SendRequestExtra{
    AdditionalNodes: &extraNodes,
})
```

The `SendRequestExtra` struct:

```go
type SendRequestExtra struct {
    ID              types.MessageID     // Optional custom message ID
    Peer            bool                // Protocol-level peer messages only
    Timeout         time.Duration       // Response timeout (default: 75s)
    MediaHandle     string              // Newsletter media handle
    AdditionalNodes *[]waBinary.Node    // Extra stanza nodes ← THIS IS THE KEY
}
```

---

## Buttons Message

### Buttons Input Parameters

To send a buttons message you need these parameters:

| Parameter   | Type     | Required | Description                                                              |
|-------------|----------|----------|--------------------------------------------------------------------------|
| `to`        | string   | **Yes**  | Recipient JID (e.g. `5511999999999@s.whatsapp.net` or group JID)         |
| `text`      | string   | **Yes**  | Message body text displayed above the buttons                            |
| `title`     | string   | No       | Header text above the body. If present, `HeaderType` = `TEXT`            |
| `footer`    | string   | No       | Footer text displayed below the buttons                                  |
| `buttons`   | array    | **Yes**  | Array of button objects (max 3). Each needs at least a `title`           |

**Button object fields:**

| Field       | Type     | Required | Description                                                              |
|-------------|----------|----------|--------------------------------------------------------------------------|
| `title`     | string   | **Yes**  | Display text on the button                                               |
| `id`        | string   | No       | Unique button ID. Defaults to the title if empty                         |

### Buttons Protobuf Construction

Step-by-step construction of the protobuf message:

```go
func buildButtonsMessage(
    body string,       // required: message text
    title string,      // optional: header text
    footer string,     // optional: footer text
    buttons []Button,  // required: at least 1 button
) *waProto.Message {

    // 1. Build each button
    protoButtons := make([]*waProto.ButtonsMessage_Button, 0, len(buttons))
    for _, btn := range buttons {
        buttonID := btn.ID
        if buttonID == "" {
            buttonID = btn.Title  // fallback: use title as ID
        }

        protoButtons = append(protoButtons, &waProto.ButtonsMessage_Button{
            ButtonID: proto.String(buttonID),
            ButtonText: &waProto.ButtonsMessage_Button_ButtonText{
                DisplayText: proto.String(btn.Title),
            },
            Type:           waProto.ButtonsMessage_Button_RESPONSE.Enum(),
            NativeFlowInfo: &waProto.ButtonsMessage_Button_NativeFlowInfo{},
        })
    }

    // 2. Build the ButtonsMessage
    buttonsMsg := &waProto.ButtonsMessage{
        ContentText: proto.String(body),
        HeaderType:  waProto.ButtonsMessage_EMPTY.Enum(),
        Buttons:     protoButtons,
    }

    // 3. Optional: Add header text
    if title != "" {
        buttonsMsg.HeaderType = waProto.ButtonsMessage_TEXT.Enum()
        buttonsMsg.Header = &waProto.ButtonsMessage_Text{Text: title}
    }

    // 4. Optional: Add footer text
    if footer != "" {
        buttonsMsg.FooterText = proto.String(footer)
    }

    // 5. Wrap in FutureProofMessage (CRITICAL!)
    return &waProto.Message{
        DocumentWithCaptionMessage: &waProto.FutureProofMessage{
            Message: &waProto.Message{
                ButtonsMessage: buttonsMsg,
            },
        },
    }
}
```

**Key details per button:**

| Proto Field      | Value                                        | Notes                                           |
|------------------|----------------------------------------------|--------------------------------------------------|
| `ButtonID`       | `proto.String("btn_1")`                      | Unique identifier returned in response events    |
| `ButtonText`     | `ButtonText{DisplayText: proto.String(...)}`  | What the user sees on the button                |
| `Type`           | `ButtonsMessage_Button_RESPONSE.Enum()`       | Must be `RESPONSE` (value `1`)                  |
| `NativeFlowInfo` | `&ButtonsMessage_Button_NativeFlowInfo{}`     | Empty struct, but **required** for rendering    |

**ButtonsMessage fields:**

| Proto Field    | Value                                      | Notes                                            |
|----------------|--------------------------------------------|--------------------------------------------------|
| `ContentText`  | `proto.String("Choose an option")`         | The main body text                               |
| `HeaderType`   | `ButtonsMessage_EMPTY` or `_TEXT`          | `EMPTY` if no title, `TEXT` if title given        |
| `Header`       | `&ButtonsMessage_Text{Text: "title"}`      | Only when `HeaderType` = `TEXT`                  |
| `FooterText`   | `proto.String("footer")`                   | Optional footer                                  |
| `Buttons`      | `[]*ButtonsMessage_Button{...}`            | 1-3 buttons                                      |

### Buttons Extra Binary Nodes

The extra nodes signal this is a "native flow" interactive message:

```go
extraNodes := []waBinary.Node{{
    Tag: "biz",
    Content: []waBinary.Node{{
        Tag: "interactive",
        Attrs: waBinary.Attrs{
            "type": "native_flow",
            "v":    "1",
        },
        Content: []waBinary.Node{{
            Tag: "native_flow",
            Attrs: waBinary.Attrs{
                "v":    "9",
                "name": "mixed",
            },
        }},
    }},
}}
```

**Node tree:**
```
biz
└── interactive (type="native_flow", v="1")
    └── native_flow (v="9", name="mixed")
```

### Complete Buttons Code

```go
func sendButtons(ctx context.Context, client *whatsmeow.Client, params SendButtonsParams) (whatsmeow.SendResponse, error) {
    // Validate
    if strings.TrimSpace(params.To) == "" {
        return whatsmeow.SendResponse{}, fmt.Errorf("to is required")
    }
    if strings.TrimSpace(params.Text) == "" {
        return whatsmeow.SendResponse{}, fmt.Errorf("text is required")
    }
    if len(params.Buttons) == 0 {
        return whatsmeow.SendResponse{}, fmt.Errorf("buttons is required")
    }

    // Parse recipient JID
    chatJID, err := parseJID(params.To)
    if err != nil {
        return whatsmeow.SendResponse{}, err
    }

    // Build buttons
    protoButtons := make([]*waProto.ButtonsMessage_Button, 0, len(params.Buttons))
    for _, btn := range params.Buttons {
        title := strings.TrimSpace(btn.Title)
        if title == "" {
            continue
        }
        buttonID := strings.TrimSpace(btn.ID)
        if buttonID == "" {
            buttonID = title
        }
        protoButtons = append(protoButtons, &waProto.ButtonsMessage_Button{
            ButtonID: proto.String(buttonID),
            ButtonText: &waProto.ButtonsMessage_Button_ButtonText{
                DisplayText: proto.String(title),
            },
            Type:           waProto.ButtonsMessage_Button_RESPONSE.Enum(),
            NativeFlowInfo: &waProto.ButtonsMessage_Button_NativeFlowInfo{},
        })
    }
    if len(protoButtons) == 0 {
        return whatsmeow.SendResponse{}, fmt.Errorf("valid buttons are required")
    }

    // Build message
    buttonsMsg := &waProto.ButtonsMessage{
        ContentText: proto.String(params.Text),
        HeaderType:  waProto.ButtonsMessage_EMPTY.Enum(),
        Buttons:     protoButtons,
    }
    if params.Title != "" {
        buttonsMsg.HeaderType = waProto.ButtonsMessage_TEXT.Enum()
        buttonsMsg.Header = &waProto.ButtonsMessage_Text{Text: params.Title}
    }
    if params.Footer != "" {
        buttonsMsg.FooterText = proto.String(params.Footer)
    }

    // Wrap in FutureProofMessage
    message := &waProto.Message{
        DocumentWithCaptionMessage: &waProto.FutureProofMessage{
            Message: &waProto.Message{ButtonsMessage: buttonsMsg},
        },
    }

    // Build extra nodes
    extraNodes := []waBinary.Node{{
        Tag: "biz",
        Content: []waBinary.Node{{
            Tag: "interactive",
            Attrs: waBinary.Attrs{
                "type": "native_flow",
                "v":    "1",
            },
            Content: []waBinary.Node{{
                Tag: "native_flow",
                Attrs: waBinary.Attrs{
                    "v":    "9",
                    "name": "mixed",
                },
            }},
        }},
    }}

    // Send
    return client.SendMessage(ctx, chatJID, message, whatsmeow.SendRequestExtra{
        AdditionalNodes: &extraNodes,
    })
}
```

---

## List Message

### List Input Parameters

To send a list message you need these parameters:

| Parameter    | Type     | Required | Description                                                             |
|--------------|----------|----------|-------------------------------------------------------------------------|
| `to`         | string   | **Yes**  | Recipient JID (e.g. `5511999999999@s.whatsapp.net`)                     |
| `text`       | string   | **Yes**  | Message body / description text                                         |
| `title`      | string   | No       | Header title above the body                                             |
| `footer`     | string   | No       | Footer text below the list                                              |
| `buttonText` | string   | No       | Text on the "open list" button. Defaults to `"Select"`                  |
| `sections`   | array    | **Yes**  | Array of section objects (at least 1)                                   |

**Section object fields:**

| Field    | Type   | Required | Description                       |
|----------|--------|----------|-----------------------------------|
| `title`  | string | No       | Section header displayed above rows |
| `rows`   | array  | **Yes**  | Array of row objects (at least 1) |

**Row object fields:**

| Field         | Type   | Required | Description                                                     |
|---------------|--------|----------|-----------------------------------------------------------------|
| `title`       | string | **Yes**  | Row display text                                                |
| `id`          | string | No       | Unique row ID. Defaults to the row title if empty               |
| `description` | string | No       | Optional description shown below the title                      |

### List Protobuf Construction

```go
func buildListMessage(
    description string,   // required: body text
    title string,         // optional: header
    footer string,        // optional: footer
    buttonText string,    // optional: button label (default: "Select")
    sections []Section,   // required: at least 1 section with rows
) *waProto.Message {

    // 1. Build sections and rows
    protoSections := make([]*waProto.ListMessage_Section, 0, len(sections))
    for _, sec := range sections {
        rows := make([]*waProto.ListMessage_Row, 0, len(sec.Rows))
        for _, row := range sec.Rows {
            rowTitle := strings.TrimSpace(row.Title)
            if rowTitle == "" {
                continue
            }
            rowID := strings.TrimSpace(row.ID)
            if rowID == "" {
                rowID = rowTitle  // fallback: use title as ID
            }
            protoRow := &waProto.ListMessage_Row{
                RowID: proto.String(rowID),
                Title: proto.String(rowTitle),
            }
            if row.Description != "" {
                protoRow.Description = proto.String(row.Description)
            }
            rows = append(rows, protoRow)
        }
        if len(rows) == 0 {
            continue
        }
        section := &waProto.ListMessage_Section{Rows: rows}
        if sec.Title != "" {
            section.Title = proto.String(sec.Title)
        }
        protoSections = append(protoSections, section)
    }

    // 2. Build ListMessage
    if strings.TrimSpace(buttonText) == "" {
        buttonText = "Select"
    }
    listMsg := &waProto.ListMessage{
        Description: proto.String(description),
        ButtonText:  proto.String(buttonText),
        ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
        Sections:    protoSections,
    }
    if title != "" {
        listMsg.Title = proto.String(title)
    }
    if footer != "" {
        listMsg.FooterText = proto.String(footer)
    }

    // 3. Wrap in FutureProofMessage (CRITICAL!)
    return &waProto.Message{
        DocumentWithCaptionMessage: &waProto.FutureProofMessage{
            Message: &waProto.Message{
                ListMessage: listMsg,
            },
        },
    }
}
```

**ListMessage fields:**

| Proto Field    | Value                                         | Notes                                    |
|----------------|-----------------------------------------------|------------------------------------------|
| `Description`  | `proto.String("body text")`                   | Main body text                           |
| `ButtonText`   | `proto.String("View Options")`                | Label on the "open list" button          |
| `ListType`     | `ListMessage_SINGLE_SELECT.Enum()`            | Always `SINGLE_SELECT` (value `1`)       |
| `Title`        | `proto.String("header")`                      | Optional header title                    |
| `FooterText`   | `proto.String("footer")`                      | Optional footer                          |
| `Sections`     | `[]*ListMessage_Section{...}`                 | At least 1 section                       |

**Section fields:**

| Proto Field | Value                              | Notes                    |
|-------------|------------------------------------|--------------------------|
| `Title`     | `proto.String("Category A")`       | Optional section header  |
| `Rows`      | `[]*ListMessage_Row{...}`          | At least 1 row           |

**Row fields:**

| Proto Field    | Value                             | Notes                        |
|----------------|-----------------------------------|------------------------------|
| `RowID`        | `proto.String("row_1")`           | Unique ID for the row        |
| `Title`        | `proto.String("Service 1")`       | Display text (required)      |
| `Description`  | `proto.String("Description")`     | Optional description         |

### List Extra Binary Nodes

The extra nodes signal this is a list-type interactive message:

```go
extraNodes := []waBinary.Node{{
    Tag: "biz",
    Content: []waBinary.Node{{
        Tag: "list",
        Attrs: waBinary.Attrs{
            "type": "product_list",
            "v":    "2",
        },
    }},
}}
```

**Node tree:**
```
biz
└── list (type="product_list", v="2")
```

### Complete List Code

```go
func sendList(ctx context.Context, client *whatsmeow.Client, params SendListParams) (whatsmeow.SendResponse, error) {
    // Validate
    if strings.TrimSpace(params.To) == "" {
        return whatsmeow.SendResponse{}, fmt.Errorf("to is required")
    }
    if strings.TrimSpace(params.Text) == "" {
        return whatsmeow.SendResponse{}, fmt.Errorf("text is required")
    }
    if len(params.Sections) == 0 {
        return whatsmeow.SendResponse{}, fmt.Errorf("sections is required")
    }

    // Parse recipient JID
    chatJID, err := parseJID(params.To)
    if err != nil {
        return whatsmeow.SendResponse{}, err
    }

    // Build sections
    protoSections := make([]*waProto.ListMessage_Section, 0, len(params.Sections))
    for _, sec := range params.Sections {
        rows := make([]*waProto.ListMessage_Row, 0, len(sec.Rows))
        for _, row := range sec.Rows {
            rowTitle := strings.TrimSpace(row.Title)
            if rowTitle == "" {
                continue
            }
            rowID := strings.TrimSpace(row.ID)
            if rowID == "" {
                rowID = rowTitle
            }
            protoRow := &waProto.ListMessage_Row{
                RowID: proto.String(rowID),
                Title: proto.String(rowTitle),
            }
            if desc := strings.TrimSpace(row.Description); desc != "" {
                protoRow.Description = proto.String(desc)
            }
            rows = append(rows, protoRow)
        }
        if len(rows) == 0 {
            continue
        }
        section := &waProto.ListMessage_Section{Rows: rows}
        if secTitle := strings.TrimSpace(sec.Title); secTitle != "" {
            section.Title = proto.String(secTitle)
        }
        protoSections = append(protoSections, section)
    }
    if len(protoSections) == 0 {
        return whatsmeow.SendResponse{}, fmt.Errorf("valid sections are required")
    }

    // Build message
    buttonText := strings.TrimSpace(params.ButtonText)
    if buttonText == "" {
        buttonText = "Select"
    }
    listMsg := &waProto.ListMessage{
        Description: proto.String(params.Text),
        ButtonText:  proto.String(buttonText),
        ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
        Sections:    protoSections,
    }
    if params.Title != "" {
        listMsg.Title = proto.String(params.Title)
    }
    if params.Footer != "" {
        listMsg.FooterText = proto.String(params.Footer)
    }

    // Wrap in FutureProofMessage
    message := &waProto.Message{
        DocumentWithCaptionMessage: &waProto.FutureProofMessage{
            Message: &waProto.Message{ListMessage: listMsg},
        },
    }

    // Build extra nodes
    extraNodes := []waBinary.Node{{
        Tag: "biz",
        Content: []waBinary.Node{{
            Tag: "list",
            Attrs: waBinary.Attrs{
                "type": "product_list",
                "v":    "2",
            },
        }},
    }}

    // Send
    return client.SendMessage(ctx, chatJID, message, whatsmeow.SendRequestExtra{
        AdditionalNodes: &extraNodes,
    })
}
```

---

## Sending the Message

Both buttons and lists use `SendMessage` with `SendRequestExtra`:

```go
resp, err := client.SendMessage(ctx, chatJID, message, whatsmeow.SendRequestExtra{
    AdditionalNodes: &extraNodes,
})
if err != nil {
    // Handle send error
    return err
}

// resp.ID        → message ID (string)
// resp.Timestamp → server timestamp (time.Time)
```

### SendResponse

```go
type SendResponse struct {
    Timestamp    time.Time           // Server timestamp
    ID           types.MessageID     // Message ID
    ServerID     types.MessageServerID // Newsletter only
    DebugTimings MessageDebugTimings // Debug info
    Sender       types.JID           // Sender identity
}
```

---

## JID Parsing

WhatsApp uses JID (Jabber ID) to identify chats. Here's how to parse them:

```go
func parseJID(rawJID string) (waTypes.JID, error) {
    // Already contains @ → parse directly
    if strings.Contains(rawJID, "@") {
        jid, err := waTypes.ParseJID(rawJID)
        if err != nil {
            return waTypes.JID{}, fmt.Errorf("invalid JID: %w", err)
        }
        return jid, nil
    }

    // Number only → assume individual chat
    // Strip non-digits, add @s.whatsapp.net
    cleaned := stripNonDigits(rawJID)
    return waTypes.NewJID(cleaned, waTypes.DefaultUserServer), nil
}

func stripNonDigits(s string) string {
    var b strings.Builder
    for _, c := range s {
        if c >= '0' && c <= '9' {
            b.WriteRune(c)
        }
    }
    return b.String()
}
```

**JID formats:**

| Type       | Format                                | Example                                  |
|------------|---------------------------------------|------------------------------------------|
| Individual | `{phone}@s.whatsapp.net`              | `5511999999999@s.whatsapp.net`           |
| Group      | `{groupid}@g.us`                      | `120363123456789@g.us`                   |
| Broadcast  | `status@broadcast`                    | `status@broadcast`                       |

---

## Validation Rules

### Buttons

| Rule                           | Error Code         | Error Message                |
|--------------------------------|--------------------|------------------------------|
| `to` is empty                  | VALIDATION_ERROR   | "to is required"             |
| `text` is empty                | VALIDATION_ERROR   | "text is required"           |
| `buttons` array is empty/nil   | VALIDATION_ERROR   | "buttons is required"        |
| All buttons have empty titles  | VALIDATION_ERROR   | "valid buttons are required" |

**Button ID fallback:** if `id` is empty, the button title is used as ID.

### Lists

| Rule                           | Error Code         | Error Message                  |
|--------------------------------|--------------------|--------------------------------|
| `to` is empty                  | VALIDATION_ERROR   | "to is required"               |
| `text` is empty                | VALIDATION_ERROR   | "text is required"             |
| `sections` array is empty/nil  | VALIDATION_ERROR   | "sections is required"         |
| All sections/rows are invalid  | VALIDATION_ERROR   | "valid sections are required"  |

**Row ID fallback:** if `id` is empty, the row title is used as ID.

---

## Common Pitfalls

### 1. Missing FutureProofMessage Wrapper

**Wrong** — message sent but not rendered:
```go
// ❌ This will NOT show buttons on the recipient's phone
msg := &waProto.Message{
    ButtonsMessage: buttonsMsg,
}
```

**Correct:**
```go
// ✅ Wrapped in DocumentWithCaptionMessage + FutureProofMessage
msg := &waProto.Message{
    DocumentWithCaptionMessage: &waProto.FutureProofMessage{
        Message: &waProto.Message{ButtonsMessage: buttonsMsg},
    },
}
```

### 2. Missing AdditionalNodes

**Wrong** — server may reject or mishandle:
```go
// ❌ Without extra nodes, the WhatsApp server won't process it correctly
resp, err := client.SendMessage(ctx, jid, msg)
```

**Correct:**
```go
// ✅ Extra nodes tell the server this is an interactive message
resp, err := client.SendMessage(ctx, jid, msg, whatsmeow.SendRequestExtra{
    AdditionalNodes: &extraNodes,
})
```

### 3. Missing NativeFlowInfo on Buttons

Each button **must** have an empty `NativeFlowInfo` struct:

```go
// ❌ Missing NativeFlowInfo
&waProto.ButtonsMessage_Button{
    ButtonID:   proto.String("id"),
    ButtonText: &waProto.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("Click")},
    Type:       waProto.ButtonsMessage_Button_RESPONSE.Enum(),
}

// ✅ With empty NativeFlowInfo
&waProto.ButtonsMessage_Button{
    ButtonID:   proto.String("id"),
    ButtonText: &waProto.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("Click")},
    Type:       waProto.ButtonsMessage_Button_RESPONSE.Enum(),
    NativeFlowInfo: &waProto.ButtonsMessage_Button_NativeFlowInfo{},
}
```

### 4. Wrong Button Type

Always use `RESPONSE`. Other values:
- `UNKNOWN` (0) — don't use
- `RESPONSE` (1) — **correct** for interactive buttons
- `NATIVE_FLOW` (2) — for advanced native flow actions (not standard buttons)

### 5. Missing ListType for Lists

Always set `ListType` to `SINGLE_SELECT`:

```go
listMsg := &waProto.ListMessage{
    // ...
    ListType: waProto.ListMessage_SINGLE_SELECT.Enum(),  // Required!
}
```

### 6. Different Extra Nodes for Buttons vs Lists

**Buttons** use `interactive` with `native_flow`:
```go
// biz > interactive(type=native_flow, v=1) > native_flow(v=9, name=mixed)
```

**Lists** use `list` with `product_list`:
```go
// biz > list(type=product_list, v=2)
```

**Do NOT mix them** — using list nodes for buttons (or vice versa) will cause delivery failure.

---

## Testing

Validation can be tested without a connected WhatsApp client by calling the builder functions
directly. Here's the test pattern used in gomib:

```go
func TestSendButtonsValidation(t *testing.T) {
    tests := []struct {
        name        string
        to          string
        text        string
        buttons     []Button
        expectError string
    }{
        {
            name:        "missing to",
            to:          "",
            text:        "hello",
            buttons:     []Button{{Title: "A"}},
            expectError: "to is required",
        },
        {
            name:        "missing text",
            to:          "5511999999999@s.whatsapp.net",
            text:        "",
            buttons:     []Button{{Title: "A"}},
            expectError: "text is required",
        },
        {
            name:        "missing buttons",
            to:          "5511999999999@s.whatsapp.net",
            text:        "hello",
            buttons:     nil,
            expectError: "buttons is required",
        },
        {
            name:        "all empty button titles",
            to:          "5511999999999@s.whatsapp.net",
            text:        "hello",
            buttons:     []Button{{Title: ""}},
            expectError: "valid buttons are required",
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            _, err := sendButtons(context.Background(), nil, SendButtonsParams{
                To:      tc.to,
                Text:    tc.text,
                Buttons: tc.buttons,
            })
            if err == nil || !strings.Contains(err.Error(), tc.expectError) {
                t.Errorf("expected error containing %q, got %v", tc.expectError, err)
            }
        })
    }
}

func TestSendListValidation(t *testing.T) {
    tests := []struct {
        name        string
        to          string
        text        string
        sections    []Section
        expectError string
    }{
        {
            name:        "missing to",
            to:          "",
            text:        "hello",
            sections:    []Section{{Rows: []Row{{Title: "A"}}}},
            expectError: "to is required",
        },
        {
            name:        "missing text",
            to:          "5511999999999@s.whatsapp.net",
            text:        "",
            sections:    []Section{{Rows: []Row{{Title: "A"}}}},
            expectError: "text is required",
        },
        {
            name:        "missing sections",
            to:          "5511999999999@s.whatsapp.net",
            text:        "hello",
            sections:    nil,
            expectError: "sections is required",
        },
        {
            name:        "all empty row titles",
            to:          "5511999999999@s.whatsapp.net",
            text:        "hello",
            sections:    []Section{{Rows: []Row{{Title: ""}}}},
            expectError: "valid sections are required",
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            _, err := sendList(context.Background(), nil, SendListParams{
                To:       tc.to,
                Text:     tc.text,
                Sections: tc.sections,
            })
            if err == nil || !strings.Contains(err.Error(), tc.expectError) {
                t.Errorf("expected error containing %q, got %v", tc.expectError, err)
            }
        })
    }
}
```

---

## Full Working Example

### Type Definitions

```go
type Button struct {
    ID    string
    Title string
}

type Row struct {
    ID          string
    Title       string
    Description string
}

type Section struct {
    Title string
    Rows  []Row
}

type SendButtonsParams struct {
    To      string
    Text    string
    Title   string
    Footer  string
    Buttons []Button
}

type SendListParams struct {
    To         string
    Text       string
    Title      string
    Footer     string
    ButtonText string
    Sections   []Section
}
```

### Usage

```go
// --- Buttons ---
resp, err := sendButtons(ctx, client, SendButtonsParams{
    To:     "5511999999999@s.whatsapp.net",
    Text:   "What would you like to do?",
    Title:  "Quick Actions",
    Footer: "Tap a button to continue",
    Buttons: []Button{
        {ID: "help",    Title: "Help"},
        {ID: "pricing", Title: "Pricing"},
        {ID: "contact", Title: "Contact Us"},
    },
})

// --- List ---
resp, err := sendList(ctx, client, SendListParams{
    To:         "5511999999999@s.whatsapp.net",
    Text:       "Browse our catalog:",
    Title:      "Product Catalog",
    Footer:     "Tap 'View Products' to see options",
    ButtonText: "View Products",
    Sections: []Section{
        {
            Title: "Electronics",
            Rows: []Row{
                {ID: "phone",  Title: "Smartphones",  Description: "Latest models"},
                {ID: "laptop", Title: "Laptops",      Description: "Work & gaming"},
            },
        },
        {
            Title: "Accessories",
            Rows: []Row{
                {ID: "case",    Title: "Phone Cases"},
                {ID: "charger", Title: "Chargers", Description: "Fast charging"},
            },
        },
    },
})
```

---

## Quick Reference

| Feature          | Buttons                                        | Lists                                         |
|------------------|------------------------------------------------|-----------------------------------------------|
| **Proto type**   | `ButtonsMessage`                               | `ListMessage`                                 |
| **Wrapper**      | `DocumentWithCaptionMessage.FutureProofMessage` | `DocumentWithCaptionMessage.FutureProofMessage` |
| **Extra node**   | `biz > interactive > native_flow`              | `biz > list`                                  |
| **Node type**    | `native_flow` (v=1)                            | `product_list` (v=2)                          |
| **Inner node**   | `native_flow` (v=9, name=mixed)                | *(none)*                                      |
| **Item type**    | `ButtonsMessage_Button` (RESPONSE)             | `ListMessage_Section` → `ListMessage_Row`     |
| **Select type**  | N/A                                            | `SINGLE_SELECT`                               |
| **Max items**    | 3 buttons                                      | No hard limit (practical: ~10 rows/section)   |
| **Has footer?**  | Yes (optional)                                 | Yes (optional)                                |
| **Has header?**  | Yes (optional, changes HeaderType)             | Yes (optional)                                |
| **Button label** | N/A                                            | `ButtonText` (default: "Select")              |

---

## Resources

- [whatsmeow Repository](https://github.com/tulir/whatsmeow)
- [whatsmeow GoDoc](https://pkg.go.dev/go.mau.fi/whatsmeow)
- [waE2E Protobuf Schema](https://pkg.go.dev/go.mau.fi/whatsmeow/proto/waE2E)
- [waBinary Node Types](https://pkg.go.dev/go.mau.fi/whatsmeow/binary)
