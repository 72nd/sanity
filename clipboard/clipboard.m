#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>
#include "clipboard.h"

char *sanityClipboardFiles(void) {
	@autoreleasepool {
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		NSDictionary *opts = @{NSPasteboardURLReadingFileURLsOnlyKey : @YES};
		NSArray *urls = [pb readObjectsForClasses:@[ [NSURL class] ] options:opts];
		NSMutableArray<NSString *> *paths = [NSMutableArray array];
		for (NSURL *u in urls) {
			if ([u isFileURL] && u.path.length > 0) {
				[paths addObject:u.path];
			}
		}
		if (paths.count == 0) {
			return NULL;
		}
		return strdup([[paths componentsJoinedByString:@"\n"] UTF8String]);
	}
}

int sanityClipboardSetFile(const char *path) {
	@autoreleasepool {
		NSURL *u = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb clearContents];
		return [pb writeObjects:@[ u ]] ? 1 : 0;
	}
}
