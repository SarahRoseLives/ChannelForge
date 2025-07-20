sub ShowGridScreen()
    if m.screenStack = invalid then m.screenStack = []
    grid = CreateObject("roSGNode", "GridScreen")
    grid.ObserveField("drillRequest", "OnDrillRequest")
    grid.ObserveField("closeRequest", "OnGridCloseRequest")
    ShowScreen(grid)
    m.screenStack.Push(grid)
    m.GridScreen = grid
end sub

sub OnDrillRequest()
    contentNode = m.GridScreen.drillRequest
    if contentNode <> invalid then
        newGrid = CreateObject("roSGNode", "GridScreen")
        newGrid.content = contentNode
        newGrid.ObserveField("drillRequest", "OnDrillRequest")
        newGrid.ObserveField("closeRequest", "OnGridCloseRequest")
        ShowScreen(newGrid)
        m.screenStack.Push(newGrid)
        m.GridScreen = newGrid
        m.GridScreen.drillRequest = invalid
    end if
end sub

sub OnGridCloseRequest()
    if m.screenStack.Count() > 1
        curr = m.screenStack.Pop()
        CloseScreen(curr)
        m.GridScreen = m.screenStack.Peek()
        m.GridScreen.SetFocus(true)
    else
        ' AT ROOT: DO NOT remove last screen!
        if m.GridScreen <> invalid
            m.GridScreen.SetFocus(true)
            ' Try to focus rowList directly, too, as extra insurance
            rowList = m.GridScreen.FindNode("rowList")
            if rowList <> invalid
                rowList.SetFocus(true)
            end if
        end if
    end if
end sub