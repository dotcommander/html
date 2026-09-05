# Notes from the field

A durable record begins with careful observation. These notes demonstrate headings,
source links, code, and margin alerts in both reading layouts.

## Before setting out

Choose a route, pack the essentials, and write down the question you hope to answer.
Leave enough time to stop and look closely.

> [!NOTE]
> A useful observation records where, when, and under what conditions it was made.

> [!TIP]
> Consecutive notes each keep their own space. Their source order is preserved in
> the margin, on smaller screens, and on paper.

## At the observation point

Record the landscape before interpreting it. Distinguish what you can see from
what you think it means, and keep a link to the [original notes](#before-setting-out).

> [!IMPORTANT]
> A longer margin note can hold a complete checklist:
>
> - Record the time and weather.
> - Describe the viewpoint and direction.
> - Note the instrument and its settings.
> - Preserve the original measurement.
> - Record uncertainty alongside the value.
> - Link photographs to their observation.
> - Keep conflicting readings for later review.
>
> Tall notes reserve enough space to remain readable, including when another note
> follows immediately. This text is never copied into a separate sidebar.

> [!WARNING]
> Keep an unedited copy of the measurements before making corrections.

## A small measurement log

| Location | Wind | Temperature |
|:--|--:|--:|
| Ridge | 12 km/h | 18 °C |
| Meadow | 5 km/h | 21 °C |
| Stream | 2 km/h | 16 °C |

```go
type Observation struct {
    Location string
    Notes    string
}
```

## Reading the record

Compare observations, preserve uncertainty, and return to the same point when the
conditions change. A record with no conclusion can still be useful evidence.

> A regular blockquote stays in the text. Only Markdown alerts become margin notes.

## Next visit

- Bring a spare notebook.
- Recheck the stream temperature.
- Photograph the ridge from the same viewpoint.
