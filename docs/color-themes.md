# Color theme migration

The bubbletint v2.0.4 dependency update replaces the theme catalog with newer
data from iTerm2-Color-Schemes. This expands the catalog but also changes or
removes some theme IDs that were available with bubbletint v2.0.2.

Run `kongctl list themes --color-theme auto` to browse available themes.
Select one using `--color-theme <name>`, your profile's `color-theme` setting,
or its environment variable, such as `KONGCTL_DEFAULT_COLOR_THEME`.

## Renamed themes

These old names remain supported as silent aliases. The replacements preserve
all colors used by kongctl. New configurations should use the current name.

| Previous ID | Current ID |
| --- | --- |
| `coolnight` | `cool_night` |
| `solarized_dark___patched` | `solarized_dark_patched` |
| `space_gray_eighties` | `spacegray_eighties` |
| `space_gray_eighties_dull` | `spacegray_eighties_dull` |
| `tokyonight` | `tokyo_night` |
| `tokyonight_day` | `tokyo_night_day` |

## Removed themes

The following old IDs have no verified equivalent in the new catalog:

```text
bubbles
builtin_solarized_dark
builtin_solarized_light
contrast_light
crystal_violet
cyber_cube
cyber_punk_2077
ganyu
github
glorious
h_4_rithd
h_4_rithd_com
juicy_colors
kanagawa
liquid_carbon_transparent_inverse
monokai_cmder
monokai_pro_filter_octagon
monokai_pro_filter_ristretto
moonlight_ii
one_dark
one_star
primer
qb_64_super_dark_blue
retrowave
sonoran_gothic
sonoran_sunrise
space_gray
tokyo_night_light
tokyonight_storm
unholy
zeonica
```

If you select one of these IDs, kongctl warns on STDERR and uses `auto` for
that invocation. Commands continue, and warnings do not appear in JSON or YAML
output on STDOUT. The `auto` theme selects `kong-dark` or `kong-light` based on
your terminal background. Your configuration file is not rewritten; update
your configured theme to stop the warning.

In `kongctl view`, the warning also appears in the TUI status area. It stays
visible during navigation and resource loading until you select another theme
with `t` or `T`. The status area reminds you to update `color-theme` in your
configuration to persist your choice.

This fallback applies only to these known removed IDs. Other unknown theme
names produce an error with instructions for listing available themes.
