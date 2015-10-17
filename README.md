# webrip
A simple golang based webcrawler designed to scrape a page for specific filetypes

For example, you want to scrape a full web directory and only grab files of type .tar.gz

Optional regular expressions can also be applied.  So maybe you want all .zip files but only if the complete
URL matches the regular expression .+/TimsStuff/package/.+ which would mean that you would only grab .zip files
out of urls which have '/TimsStuff/package/' in them.
