' ********** Copyright 2020 Roku Corp.  All Rights Reserved. **********

sub RunContentTask()
    m.contentTask = CreateObject("roSGNode", "MainLoaderTask") ' create task for feed retrieving
    m.contentTask.ObserveField("content", "OnMainContentLoaded")
    m.contentTask.control = "run"
    m.loadingIndicator.visible = true
end sub

sub OnMainContentLoaded()
    if m.GridScreen <> invalid
        m.GridScreen.SetFocus(true)
        m.GridScreen.content = m.contentTask.content
    end if
    m.loadingIndicator.visible = false
end sub