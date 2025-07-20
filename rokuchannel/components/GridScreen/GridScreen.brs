sub Init()
    m.rowList = m.top.FindNode("rowList")
    m.rowList.SetFocus(true)
    m.descriptionLabel = m.top.FindNode("descriptionLabel")
    m.titleLabel = m.top.FindNode("titleLabel")
    m.rowList.ObserveField("rowItemFocused", "OnItemFocused")
    m.rowList.ObserveField("itemSelected", "OnItemSelected")
    m.videoPlayer = invalid
end sub

sub OnItemFocused()
    focusedIndex = m.rowList.rowItemFocused
    row = m.rowList.content.GetChild(focusedIndex[0])
    item = row.GetChild(focusedIndex[1])
    m.titleLabel.text = item.title
    if item.HasField("description") and item.description <> invalid and item.description <> "" then
        m.descriptionLabel.text = item.description
    else
        m.descriptionLabel.text = ""
    end if
    if item.HasField("length") and item.length <> invalid then
        m.titleLabel.text += " | " + GetTime(item.length)
    end if
end sub

sub OnItemSelected()
    focusedIndex = m.rowList.rowItemFocused
    row = m.rowList.content.GetChild(focusedIndex[0])
    item = row.GetChild(focusedIndex[1])

    if item.GetChildCount() > 0 then
        isEpisodeList = true
        for i = 0 to item.GetChildCount() - 1
            child = item.GetChild(i)
            if child.GetChildCount() > 0 or not child.HasField("url") then
                isEpisodeList = false
                exit for
            end if
        end for
        if isEpisodeList then
            episodeRows = []
            for i = 0 to item.GetChildCount()-1
                ep = item.GetChild(i)
                rowNode = CreateObject("roSGNode", "ContentNode")
                rowNode.title = ep.title
                rowNode.description = ep.description
                rowNode.hdPosterUrl = ep.hdPosterUrl
                rowNode.AppendChild(ep)
                episodeRows.Push(rowNode)
            end for
            wrapperNode = CreateObject("roSGNode", "ContentNode")
            wrapperNode.Update({children: episodeRows}, true)
            m.top.drillRequest = wrapperNode
            return
        else
            m.top.drillRequest = item
            return
        end if
    end if

    if item.GetChildCount() = 0 and item.url <> invalid and item.url <> "" then
        ShowVideoPlayer(item.url, item.streamFormat, item.title)
        return
    end if
end sub

sub ShowVideoPlayer(url as String, streamFormat = "mp4" as String, title = "Video" as String)
    if m.videoPlayer <> invalid
        m.top.RemoveChild(m.videoPlayer)
        m.videoPlayer = invalid
    end if
    m.videoPlayer = CreateObject("roSGNode", "Video")
    m.videoPlayer.width = 1280
    m.videoPlayer.height = 720
    m.videoPlayer.translation = [0,0]
    m.videoPlayer.observeField("state", "OnVideoPlayerStateChanged")
    m.top.appendChild(m.videoPlayer)
    videoContent = CreateObject("roSGNode", "ContentNode")
    videoContent.url = url
    videoContent.streamFormat = streamFormat
    videoContent.title = title
    m.videoPlayer.content = videoContent
    m.videoPlayer.control = "play"
end sub

sub OnVideoPlayerStateChanged()
    if m.videoPlayer.state = "done" or m.videoPlayer.state = "error"
        m.top.removeChild(m.videoPlayer)
        m.videoPlayer = invalid
    end if
end sub

function onKeyEvent(key as String, press as Boolean) as Boolean
    if press
        if m.videoPlayer <> invalid
            if key = "back"
                m.top.removeChild(m.videoPlayer)
                m.videoPlayer = invalid
                return true
            end if
        else if key = "back"
            m.top.closeRequest = true
            return true
        end if
    end if
    return false
end function

function GetTime(length as Integer) as String
    minutes = (length \ 60).ToStr()
    seconds = length MOD 60
    if seconds < 10
       seconds = "0" + seconds.ToStr()
    else
       seconds = seconds.ToStr()
    end if
    return minutes + ":" + seconds
end function