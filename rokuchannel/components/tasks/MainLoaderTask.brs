sub Init()
    m.top.functionName = "GetContent"
end sub

sub GetContent()
    xfer = CreateObject("roURLTransfer")
    xfer.SetCertificatesFile("common:/certs/ca-bundle.crt")
    xfer.SetURL("http://192.168.254.19:8080/feed.xml")
    rsp = xfer.GetToString()
    rootChildren = []

    if rsp = invalid or rsp = ""
        print "Error: No response from XML feed"
        m.top.content = invalid
        return
    end if

    xml = CreateObject("roXMLElement")
    if not xml.Parse(rsp)
        print "Error parsing XML feed: " + rsp
        m.top.content = invalid
        return
    end if

    for each category in xml.GetNamedElements("category")
        row = {}
        row.title = category@name
        row.children = []

        seriesList = category.GetNamedElements("series")
        if seriesList.Count() > 0
            for each series in seriesList
                seriesRow = {}
                seriesRow.title = series@name
                seriesRow.children = []

                seriesRow.hdPosterUrl = GetFirstText(series, "thumbnail")
                seriesRow.description = GetFirstText(series, "longDescription")
                if seriesRow.description = "" or seriesRow.description = invalid
                    seriesRow.description = GetFirstText(series, "shortDescription")
                end if

                for each season in series.GetNamedElements("season")
                    seasonRow = {}
                    seasonRow.title = season@name
                    seasonRow.children = []

                    seasonRow.hdPosterUrl = GetFirstText(season, "thumbnail")
                    seasonRow.description = GetFirstText(season, "longDescription")
                    if seasonRow.description = "" or seasonRow.description = invalid
                        seasonRow.description = GetFirstText(season, "shortDescription")
                    end if

                    for each item in season.GetNamedElements("item")
                        itemData = GetItemData(item)
                        seasonRow.children.Push(itemData)
                    end for

                    seriesRow.children.Push(seasonRow)
                end for

                row.children.Push(seriesRow)
            end for

        else
            for each item in category.GetNamedElements("item")
                itemData = GetItemData(item)
                row.children.Push(itemData)
            end for
        end if

        rootChildren.Push(row)
    end for

    contentNode = CreateObject("roSGNode", "ContentNode")
    contentNode.Update({
        children: rootChildren
    }, true)
    m.top.content = contentNode
end sub

function GetItemData(item as Object) as Object
    result = {}

    idElems = item.GetNamedElements("id")
    if idElems.Count() > 0
        result.id = idElems[0].GetText()
    else
        result.id = ""
    end if

    titleElems = item.GetNamedElements("title")
    if titleElems.Count() > 0
        result.title = titleElems[0].GetText()
    else
        result.title = ""
    end if

    descElems = item.GetNamedElements("longDescription")
    if descElems.Count() > 0
        result.description = descElems[0].GetText()
    else
        result.description = ""
    end if

    if result.description = invalid or result.description = ""
        shortDescElems = item.GetNamedElements("shortDescription")
        if shortDescElems.Count() > 0
            result.description = shortDescElems[0].GetText()
        else
            result.description = ""
        end if
    end if

    thumbElems = item.GetNamedElements("thumbnail")
    if thumbElems.Count() > 0
        result.hdPosterURL = thumbElems[0].GetText()
    else
        result.hdPosterURL = ""
    end if

    relElems = item.GetNamedElements("releaseDate")
    if relElems.Count() > 0
        result.releaseDate = relElems[0].GetText()
    else
        result.releaseDate = ""
    end if

    contentElems = item.GetNamedElements("content")
    if contentElems.Count() > 0
        videoElems = contentElems[0].GetNamedElements("video")
        if videoElems.Count() > 0
            urlElems = videoElems[0].GetNamedElements("url")
            if urlElems.Count() > 0
                result.url = urlElems[0].GetText()
            else
                result.url = ""
            end if
            formatElems = videoElems[0].GetNamedElements("streamFormat")
            if formatElems.Count() > 0
                result.streamFormat = formatElems[0].GetText()
            else
                result.streamFormat = "mp4"
            end if
            durElems = videoElems[0].GetNamedElements("duration")
            if durElems.Count() > 0
                result.length = durElems[0].GetText().ToInt()
            else
                result.length = 0
            end if
        else
            result.url = ""
            result.streamFormat = "mp4"
            result.length = 0
        end if
    else
        result.url = ""
        result.streamFormat = "mp4"
        result.length = 0
    end if

    return result
end function

function GetFirstText(parent as Object, tag as String) as String
    elems = parent.GetNamedElements(tag)
    if elems.Count() > 0
        return elems[0].GetText()
    end if
    return ""
end function