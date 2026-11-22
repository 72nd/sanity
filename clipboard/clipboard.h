#ifndef SANITY_CLIPBOARD_H
#define SANITY_CLIPBOARD_H

// sanityClipboardFiles returns the absolute paths of the file URLs on the
// general pasteboard, separated by newlines (malloc'd, free with free()), or
// NULL when there are none.
char *sanityClipboardFiles(void);

// sanityClipboardSetFile replaces the pasteboard content by one file URL.
// It reports 1 on success.
int sanityClipboardSetFile(const char *path);

#endif
